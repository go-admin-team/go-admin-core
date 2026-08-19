package config

import (
	"time"

	"github.com/go-admin-team/go-admin-core/v2/storage"
	"github.com/go-admin-team/go-admin-core/v2/storage/queue"
	redisstore "github.com/go-admin-team/go-admin-core/v2/storage/redis"
)

// Queue selects the queue backend, on the same terms as Cache: memory unless a
// redis section says otherwise.
type Queue struct {
	Redis  *RedisQueue
	Memory *QueueMemory
}

type QueueMemory struct {
	PoolSize uint
}

// RedisQueue adds the consumer-group settings to a Redis connection.
//
// The json names are what a settings file is keyed on. For what each setting
// means and what a zero value falls back to, see redisstore.QueueOptions;
// options absent here can only be set in code.
type RedisQueue struct {
	RedisOptions `yaml:",inline"`

	Group       string `yaml:"group" json:"group"`
	KeyPrefix   string `yaml:"key_prefix" json:"key_prefix"`
	MaxAttempts int    `yaml:"max_attempts" json:"max_attempts"`

	// In seconds, because a settings file cannot express a time.Duration.
	ClaimMinIdleSeconds int `yaml:"claim_min_idle_seconds" json:"claim_min_idle_seconds"`
}

var QueueConfig = new(Queue)

// Empty 空设置
func (e Queue) Empty() bool {
	return e.Memory == nil && e.Redis == nil
}

// Open builds the configured queue. Order: redis > memory.
//
// As with Cache.Open, an unreachable redis section is an error rather than a
// fallback to a queue that other instances cannot see.
func (e Queue) Open() (storage.Queue, error) {
	if e.Redis == nil {
		// Open is exported, so it cannot assume Empty was checked first.
		var size int
		if e.Memory != nil {
			size = int(e.Memory.PoolSize)
		}
		return queue.NewMemQueue(size), nil
	}

	client, err := e.Redis.connect()
	if err != nil {
		return nil, err
	}
	return redisstore.NewQueue(client, redisstore.QueueOptions{
		Group:        e.Redis.Group,
		KeyPrefix:    e.Redis.KeyPrefix,
		MaxAttempts:  e.Redis.MaxAttempts,
		ClaimMinIdle: time.Duration(e.Redis.ClaimMinIdleSeconds) * time.Second,
	}), nil
}

// Setup builds the queue adapter. Order: redis > memory.
//
// Deprecated: use Open, which returns the current contract.
//
// Note that the two branches do not behave identically, because the memory one
// stays on the older implementation: there, Register starts consuming on its
// own and retries a failed message three times, while the bridged Redis queue
// only consumes once Run is called. A call site that registers without running
// works on memory and silently consumes nothing on Redis.
func (e Queue) Setup() (storage.AdapterQueue, error) {
	if e.Redis == nil {
		var size uint
		if e.Memory != nil {
			size = e.Memory.PoolSize
		}
		return queue.NewMemory(size), nil
	}

	q, err := e.Open()
	if err != nil {
		return nil, err
	}
	return storage.LegacyQueueAdapter(q), nil
}
