package runtime

import (
	"log/slog"

	"github.com/go-admin-team/go-admin-core/v2/storage"
	"github.com/go-admin-team/go-admin-core/v2/storage/queue"
)

// NewQueue 创建对应上下文队列
//
// A nil q yields a fresh in-process queue, which is almost never what a caller
// wants: it works, but it is private to the returned handle, so a consumer
// registered through one call never sees what another publishes.
// Application.GetQueuePrefix passes a shared queue for exactly this reason.
func NewQueue(prefix string, q storage.AdapterQueue) storage.AdapterQueue {
	if q == nil {
		slog.Warn("runtime: queue created without a backing adapter; it is private to this handle and is not the configured backend",
			"prefix", prefix)
		q = queue.NewMemory(100)
	}
	return &Queue{
		prefix: prefix,
		queue:  q,
	}
}

type Queue struct {
	prefix string
	queue  storage.AdapterQueue
}

func (e *Queue) String() string {
	return e.queue.String()
}

// Register 注册消费者
func (e *Queue) Register(name string, f storage.ConsumerFunc) {
	e.queue.Register(name, f)
}

// Append 增加数据到生产者
func (e *Queue) Append(message storage.Messager) error {
	// The map has to go back through SetValues. Writing into what GetValues
	// returned worked only while the message already had one: when it did not,
	// the prefix went into a map that was attached to nothing and the message
	// was published without its tenant.
	values := message.GetValues()
	if values == nil {
		values = make(map[string]interface{})
	}
	values[storage.PrefixKey] = e.prefix
	message.SetValues(values)

	return e.queue.Append(message)
}

// Run 运行
func (e *Queue) Run() {
	e.queue.Run()
}

// Shutdown 停止
func (e *Queue) Shutdown() {
	if e.queue != nil {
		e.queue.Shutdown()
	}
}
