package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrCacheMiss reports that a key is absent.
//
// A key that never existed and a key that has expired are the same fact and
// return the same error. Callers must test with errors.Is; treating any
// non-nil error as "no data" turns a backend outage into a full cache
// bypass and stampedes the database.
var ErrCacheMiss = errors.New("storage: cache miss")

// ErrCacheClosed is returned by operations on a closed Cache.
var ErrCacheClosed = errors.New("storage: cache closed")

// Cache is the contract for a key/value cache.
//
// Implementations must honour ctx cancellation and deadlines, and must not
// swallow ctx.Err(). The context is checked first: a cancelled context yields
// its own error even on a closed Cache, so a caller that gave up is never told
// something else went wrong.
type Cache interface {
	// Get returns the value stored under key, or ErrCacheMiss when the key is
	// absent or expired. An empty string is a legal value and is returned with
	// a nil error.
	Get(ctx context.Context, key string) (string, error)

	// Set stores a value with a time to live. A ttl of zero or less means the
	// entry never expires.
	Set(ctx context.Context, key, val string, ttl time.Duration) error

	// Del removes keys. Removing an absent key is not an error.
	Del(ctx context.Context, keys ...string) error

	// Incr atomically adds delta and returns the resulting value. An absent key
	// starts from zero. delta may be negative. A key created this way has no
	// ttl; call Expire to add one. The read-modify-write must be atomic.
	Incr(ctx context.Context, key string, delta int64) (int64, error)

	// Expire resets the time to live, returning ErrCacheMiss when the key is
	// absent. A ttl of zero or less makes the entry permanent.
	Expire(ctx context.Context, key string, ttl time.Duration) error

	// Close releases the underlying resources. It is idempotent. Afterwards
	// every other method returns ErrCacheClosed, unless ctx is already done,
	// in which case the context error wins.
	io.Closer
}

// WithPrefix returns a Cache that prefixes every key, so that several
// applications can share one backend.
//
// This is a decorator rather than a method on the interface, mirroring the
// Logger decorators already used in this repository.
func WithPrefix(c Cache, prefix string) Cache {
	if prefix == "" {
		return c
	}
	return &prefixedCache{inner: c, prefix: prefix}
}

type prefixedCache struct {
	inner  Cache
	prefix string
}

func (p *prefixedCache) key(k string) string { return p.prefix + k }

func (p *prefixedCache) Get(ctx context.Context, key string) (string, error) {
	return p.inner.Get(ctx, p.key(key))
}

func (p *prefixedCache) Set(ctx context.Context, key, val string, ttl time.Duration) error {
	return p.inner.Set(ctx, p.key(key), val, ttl)
}

func (p *prefixedCache) Del(ctx context.Context, keys ...string) error {
	prefixed := make([]string, len(keys))
	for i, k := range keys {
		prefixed[i] = p.key(k)
	}
	return p.inner.Del(ctx, prefixed...)
}

func (p *prefixedCache) Incr(ctx context.Context, key string, delta int64) (int64, error) {
	return p.inner.Incr(ctx, p.key(key), delta)
}

func (p *prefixedCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return p.inner.Expire(ctx, p.key(key), ttl)
}

func (p *prefixedCache) Close() error { return p.inner.Close() }
