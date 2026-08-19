package config

import (
	"github.com/go-admin-team/go-admin-core/v2/storage"
	"github.com/go-admin-team/go-admin-core/v2/storage/cache"
	redisstore "github.com/go-admin-team/go-admin-core/v2/storage/redis"
)

// Cache selects the cache backend.
//
// Leaving every field unset is a valid configuration and yields the in-memory
// cache, so a single instance runs with nothing beside it. Redis is opt-in and
// only for deployments that run more than one instance.
type Cache struct {
	Redis  *RedisOptions
	Memory interface{}
}

// CacheConfig cache配置
var CacheConfig = new(Cache)

// Open builds the configured cache. Order: redis > memory.
//
// A redis section that cannot be reached is an error rather than a fallback:
// falling back would hand an operator who asked for a shared cache one that is
// not shared, and the symptom would only appear later, under load.
func (e Cache) Open() (storage.Cache, error) {
	if e.Redis == nil {
		return cache.NewMemCache(), nil
	}

	client, err := e.Redis.connect()
	if err != nil {
		return nil, err
	}
	return redisstore.New(client), nil
}

// Setup builds the cache adapter. Order: redis > memory.
//
// Deprecated: use Open, which returns the current contract. This returns the
// older AdapterCache so that existing call sites keep working; storage.Cache
// can report a miss and takes a context, neither of which survives the bridge.
func (e Cache) Setup() (storage.AdapterCache, error) {
	// The memory branch stays on the older implementation. Both satisfy
	// AdapterCache, but only this one preserves the retry and expiry behaviour
	// that current call sites were written against.
	if e.Redis == nil {
		return cache.NewMemory(), nil
	}

	c, err := e.Open()
	if err != nil {
		return nil, err
	}
	return storage.LegacyAdapter(c), nil
}
