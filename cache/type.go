package cache

import (
	"time"

	"github.com/bsm/redislock"
)

type Adapter interface {
	SetPrefix(string)
	Connect() error
	Get(key string) (string, error)
	Set(key string, val interface{}, expire int) error
	Del(key string) error
	HashGet(hk, key string) (string, error)
	HashDel(hk, key string) error
	Increase(key string) error
	Decrease(key string) error
	Expire(key string, dur time.Duration) error
	AdapterQueue
	AdapterLocker
}

type AdapterQueue interface {
	Append(name string, message Message) error
	Register(name string, f ConsumerFunc)
	Run()
	Shutdown()
}

type Message interface {
	SetID(string)
	SetStream(string)
	SetValues(map[string]interface{})
	GetID() string
	GetStream() string
	GetValues() map[string]interface{}
}

type ConsumerFunc func(Message) error

type AdapterLocker interface {
	Lock(key string, ttl int64, options *redislock.Options) (*redislock.Lock, error)
}
