package queue

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/go-admin-team/go-admin-core/v2/storage"
)

type queue chan storage.Messager

// NewMemory 内存模式
func NewMemory(poolNum uint) *Memory {
	return &Memory{
		queue:   new(sync.Map),
		PoolNum: poolNum,
		stop:    make(chan struct{}),
	}
}

type Memory struct {
	queue   *sync.Map
	wait    sync.WaitGroup
	mutex   sync.RWMutex
	PoolNum uint
	running bool // 标记队列是否已启动

	// closed is set by Shutdown. Append refuses from then on, so nothing new
	// arrives behind the drain.
	closed bool
	// stop is closed by Shutdown and is what tells each consumer to drain and
	// return. The channels themselves are never closed: Append publishes with
	// a non-blocking send and the retry path re-publishes, so closing
	// underneath either one panics the process during shutdown - which is
	// worse than the leak this replaces.
	stop chan struct{}
	// consumers counts the goroutines Register started, so Shutdown can wait
	// for their drains to finish rather than return while messages are still
	// buffered.
	consumers sync.WaitGroup
}

func (*Memory) String() string {
	return "memory"
}

func (m *Memory) makeQueue() queue {
	if m.PoolNum <= 0 {
		return make(queue)
	}
	return make(queue, m.PoolNum)
}

// queueFor returns the channel for a stream, creating it at most once.
//
// LoadOrStore is what makes it once. The Load-then-Store it replaces let two
// callers each miss, each build a channel, and the second overwrite the first:
// a producer that had already published to the discarded channel lost those
// messages, and a consumer registered on it stopped receiving. Register racing
// against the first Append is the ordinary case - a server registers its log
// consumers while requests are already arriving - and it dropped about one
// message in seven.
func (m *Memory) queueFor(name string) queue {
	if v, ok := m.queue.Load(name); ok {
		if q, ok := v.(queue); ok {
			return q
		}
	}
	v, _ := m.queue.LoadOrStore(name, m.makeQueue())
	q, ok := v.(queue)
	if !ok {
		// Whatever is stored is not a queue. Replace it rather than fail.
		q = m.makeQueue()
		m.queue.Store(name, q)
	}
	return q
}

func (m *Memory) Append(message storage.Messager) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if m.closed {
		return storage.ErrQueueClosed
	}
	memoryMessage := new(Message)
	memoryMessage.SetID(message.GetID())
	memoryMessage.SetStream(message.GetStream())
	memoryMessage.SetValues(message.GetValues())

	q := m.queueFor(message.GetStream())
	// 不再为每条消息起 goroutine 投递。
	//
	// 原实现中，队列满时 goroutine 会阻塞在 channel 写入上且永不退出；
	// 只要生产速度长期高于消费速度，goroutine 就会无上限累积，最终 OOM。
	// 日志走的正是这条队列，高频写日志的服务尤其容易触发。
	//
	// 改为非阻塞投递：队列满时立即丢弃该消息并返回错误，由调用方决定如何
	// 处理，而不是把压力转成不可见的 goroutine 泄漏。
	memoryMessage.SetID(uuid.New().String())
	select {
	case q <- memoryMessage:
	default:
		log.Printf("memory queue for stream %s is full, dropping message", message.GetStream())
		return fmt.Errorf("memory queue for stream %s is full", message.GetStream())
	}
	return nil
}

func (m *Memory) Register(name string, f storage.ConsumerFunc) {
	m.mutex.Lock()
	if m.closed {
		// Nothing will ever be published to it and nothing would stop it.
		m.mutex.Unlock()
		return
	}
	q := m.queueFor(name)
	stop := m.stop
	// Counted while holding the lock Shutdown takes, so a Register racing a
	// Shutdown either joins the group before Wait or is refused above.
	m.consumers.Add(1)
	m.mutex.Unlock()

	go func(out queue, gf storage.ConsumerFunc) {
		defer m.consumers.Done()
		for {
			select {
			case message := <-out:
				m.consume(out, gf, message, true)
			case <-stop:
				// Deliver what is already buffered, then stop. This is the
				// only place the messages accepted before Shutdown can still
				// be handled - the process usually exits as soon as Shutdown
				// returns, which is why Shutdown waits for this loop.
				for {
					select {
					case message := <-out:
						m.consume(out, gf, message, false)
					default:
						return
					}
				}
			}
		}
	}(q, f)
}

// consume runs one message through the handler, retrying up to three times
// while the queue is still open.
//
// retry is false during the drain. Re-publishing there would put the message
// back into the channel this loop is emptying, so the drain would either never
// finish or hold the shutdown open for the back-off sleep on every failure.
//
// The re-publish is a non-blocking send. The blocking one it replaces could
// deadlock a consumer against its own full queue - it is the only reader, so
// nothing would ever make room.
func (m *Memory) consume(out queue, gf storage.ConsumerFunc, message storage.Messager, retry bool) {
	if err := gf(message); err == nil || !retry {
		return
	}
	if message.GetErrorCount() >= 3 {
		return
	}
	message.SetErrorCount(message.GetErrorCount() + 1)
	// 每次间隔时长放大
	time.Sleep(time.Second * time.Duration(message.GetErrorCount()))
	select {
	case out <- message:
	default:
		log.Printf("memory queue for stream %s is full, dropping a retried message", message.GetStream())
	}
}

func (m *Memory) Run() {
	m.mutex.Lock()
	if m.running {
		m.mutex.Unlock()
		return // 避免重复运行
	}
	m.running = true
	m.mutex.Unlock()

	m.wait.Add(1)
	m.wait.Wait()
}

// Shutdown stops the queue and returns once the consumers have delivered what
// was already accepted.
//
// It used to release Run's wait group and return, leaving every consumer
// goroutine blocked on a channel that was never closed - one leaked per
// consumer per rebuild, and the messages still buffered were lost when the
// process exited. Waiting is the point: a caller shuts a queue down because it
// is about to stop, so anything not delivered by the time this returns is not
// delivered at all.
func (m *Memory) Shutdown() {
	m.mutex.Lock()
	if m.closed {
		m.mutex.Unlock()
		return
	}
	m.closed = true

	// 只有在运行状态才调用 Done()
	if m.running {
		m.running = false
		m.wait.Done()
	}
	stop := m.stop
	m.mutex.Unlock()

	if stop != nil {
		close(stop)
	}
	m.consumers.Wait()
}
