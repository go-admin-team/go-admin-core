package storage

import (
	"time"
)

const (
	PrefixKey = "__host"
)

// AdapterCache is the original cache contract.
//
// Deprecated: use Cache. This interface cannot report a miss (Get returns an
// empty string and a nil error for both an absent key and a stored empty
// string), carries no context, has no Close, and its Hash family shares one
// flat key space with ordinary keys. Wrap a Cache with LegacyAdapter to keep
// existing call sites working. To be removed in v2.0.0.
type AdapterCache interface {
	String() string
	Get(key string) (string, error)
	Set(key string, val interface{}, expire int) error
	Del(key string) error
	HashGet(hk, key string) (string, error)
	HashDel(hk, key string) error
	Increase(key string) error
	Decrease(key string) error
	Expire(key string, dur time.Duration) error
}

type AdapterQueue interface {
	String() string
	Append(message Messager) error
	Register(name string, f ConsumerFunc)
	Run()
	Shutdown()
}

type Messager interface {
	SetID(string)
	SetStream(string)
	SetValues(map[string]interface{})
	GetID() string
	GetStream() string
	GetValues() map[string]interface{}
	GetPrefix() string
	SetPrefix(string)
	SetErrorCount(count int)
	GetErrorCount() int
}

type ConsumerFunc func(Messager) error
