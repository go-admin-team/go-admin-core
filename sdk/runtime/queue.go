package runtime

import "github.com/go-admin-team/go-admin-core/storage"

// NewQueue 创建对应上下文队列
func NewQueue(prefix string, queue storage.AdapterQueue) storage.AdapterQueue {
	return &Queue{
		prefix: prefix,
		queue:  queue,
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
