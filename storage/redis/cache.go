// Package redis implements storage.Cache on top of Redis.
//
// It lives in its own package so that importing storage/cache does not drag in
// the Redis client. Only applications that actually use Redis pay for it.
package redis

import (
	"context"
	"errors"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/go-admin-team/go-admin-core/storage"
)

// Cache is a Redis-backed storage.Cache.
type Cache struct {
	client goredis.UniversalClient
	// owned reports whether Close should shut the client down. A client
	// supplied by the caller may be shared and is left alone.
	owned bool

	mu     sync.RWMutex
	closed bool
}

var _ storage.Cache = (*Cache)(nil)

// New returns a Cache backed by client. Close does not shut client down,
// because the caller may share it with other components.
func New(client goredis.UniversalClient) *Cache {
	return &Cache{client: client}
}

// Open connects using a Redis URL, for example
// redis://user:password@localhost:6379/0. The returned Cache owns the
// connection and closes it.
func Open(ctx context.Context, url string) (*Cache, error) {
	opts, err := goredis.ParseURL(url)
	if err != nil {
		return nil, err
	}

	client := goredis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	return &Cache{client: client, owned: true}, nil
}

// check reports whether the cache may still be used. The context is examined
// first, so a caller that gave up is never told something else went wrong.
func (c *Cache) check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return storage.ErrCacheClosed
	}
	return nil
}

func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	if err := c.check(ctx); err != nil {
		return "", err
	}

	v, err := c.client.Get(ctx, key).Result()
	if errors.Is(err, goredis.Nil) {
		return "", storage.ErrCacheMiss
	}
	return v, err
}

func (c *Cache) Set(ctx context.Context, key, val string, ttl time.Duration) error {
	if err := c.check(ctx); err != nil {
		return err
	}

	// A non-positive ttl means no expiry; redis treats 0 the same way.
	if ttl < 0 {
		ttl = 0
	}
	return c.client.Set(ctx, key, val, ttl).Err()
}

func (c *Cache) Del(ctx context.Context, keys ...string) error {
	if err := c.check(ctx); err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

func (c *Cache) Incr(ctx context.Context, key string, delta int64) (int64, error) {
	if err := c.check(ctx); err != nil {
		return 0, err
	}
	// INCRBY is atomic and creates the key at zero when absent.
	return c.client.IncrBy(ctx, key, delta).Result()
}

func (c *Cache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if err := c.check(ctx); err != nil {
		return err
	}

	// A non-positive ttl clears the expiry. EXPIRE with such a value would
	// delete the key instead, so PERSIST is the correct command.
	if ttl <= 0 {
		ok, err := c.client.Persist(ctx, key).Result()
		if err != nil {
			return err
		}
		if !ok {
			// PERSIST also reports false for a key that has no ttl, so the
			// key's existence has to be checked separately.
			return c.assertExists(ctx, key)
		}
		return nil
	}

	// PEXPIRE, not EXPIRE: the latter has one-second granularity and silently
	// rounds a sub-second ttl up, which the memory implementation does not do.
	ok, err := c.client.PExpire(ctx, key, ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return storage.ErrCacheMiss
	}
	return nil
}

func (c *Cache) assertExists(ctx context.Context, key string) error {
	n, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return err
	}
	if n == 0 {
		return storage.ErrCacheMiss
	}
	return nil
}

// Close marks the cache unusable and, when it owns the client, shuts it down.
//
// The lock is released before the client is closed: draining the connection
// pool can block, and concurrent callers should get ErrCacheClosed straight
// away rather than queueing behind the shutdown.
func (c *Cache) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	owned := c.owned
	c.mu.Unlock()

	if owned {
		return c.client.Close()
	}
	return nil
}
