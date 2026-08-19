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
	}
}

type Memory struct {
	queue   *sync.Map
	wait    sync.WaitGroup
	mutex   sync.RWMutex
	PoolNum uint
	running bool // 标记队列是否已启动
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

func (m *Memory) Append(message storage.Messager) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	memoryMessage := new(Message)
	memoryMessage.SetID(message.GetID())
	memoryMessage.SetStream(message.GetStream())
	memoryMessage.SetValues(message.GetValues())

	v, ok := m.queue.Load(message.GetStream())

	if !ok {
		v = m.makeQueue()
		m.queue.Store(message.GetStream(), v)
	}

	var q queue
	switch v.(type) {
	case queue:
		q = v.(queue)
	default:
		q = m.makeQueue()
		m.queue.Store(message.GetStream(), q)
	}
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
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	v, ok := m.queue.Load(name)
	if !ok {
		v = m.makeQueue()
		m.queue.Store(name, v)
	}
	var q queue
	switch v.(type) {
	case queue:
		q = v.(queue)
	default:
		q = m.makeQueue()
		m.queue.Store(name, q)
	}
	go func(out queue, gf storage.ConsumerFunc) {
		var err error
		for message := range q {
			err = gf(message)
			if err != nil {
				if message.GetErrorCount() < 3 {
					message.SetErrorCount(message.GetErrorCount() + 1)
					// 每次间隔时长放大
					i := time.Second * time.Duration(message.GetErrorCount())
					time.Sleep(i)
					out <- message
				}
				err = nil
			}
		}
	}(q, f)
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

func (m *Memory) Shutdown() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 只有在运行状态才调用 Done()
	if m.running {
		m.running = false
		m.wait.Done()
	}
}
