package runtime

import (
	"log/slog"

	"github.com/go-admin-team/go-admin-core/storage"
	"github.com/go-admin-team/go-admin-core/storage/queue"
)

// NewQueue 创建对应上下文队列
//
// A nil q yields a fresh in-process queue, which is almost never what a caller
// wants: two calls produce two unconnected queues, so a consumer registered on
// one never sees what is published to the other. Application.GetQueuePrefix
// passes a shared queue for exactly this reason.
func NewQueue(prefix string, q storage.AdapterQueue) storage.AdapterQueue {
	if q == nil {
		slog.Warn("runtime: queue created without a backing adapter; it is connected to nothing else",
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
	values := message.GetValues()
	if values == nil {
		values = make(map[string]interface{})
	}
	values[storage.PrefixKey] = e.prefix
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
