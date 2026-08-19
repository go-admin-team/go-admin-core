package cache

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/go-admin-team/go-admin-core/v2/storage"
)

const defaultSweepInterval = time.Minute

type entry struct {
	value string
	// expiresAt is the zero time when the entry never expires.
	expiresAt time.Time
}

func (e entry) expired(now time.Time) bool {
	return !e.expiresAt.IsZero() && e.expiresAt.Before(now)
}

// MemCache is an in-process storage.Cache.
//
// A single mutex guards the map. Incr must be atomic, and sync.Map cannot
// express a read-modify-write, which is how the previous implementation lost
// updates under concurrency.
type MemCache struct {
	mu     sync.Mutex
	items  map[string]entry
	closed bool

	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

var _ storage.Cache = (*MemCache)(nil)

// NewMemCache returns a cache that sweeps expired entries every
// defaultSweepInterval. Call Close when the instance is not process-wide.
// String identifies the backend, which is what the deprecated AdapterCache
// interface reports through storage.LegacyAdapter.
func (c *MemCache) String() string { return "memory" }

func NewMemCache() *MemCache {
	return NewMemCacheWithSweep(defaultSweepInterval)
}

// NewMemCacheWithSweep sets the sweep interval. Zero or less starts no
// sweeper, leaving expiry entirely lazy.
func NewMemCacheWithSweep(interval time.Duration) *MemCache {
	m := &MemCache{
		items: make(map[string]entry),
		stop:  make(chan struct{}),
	}
	if interval > 0 {
		m.wg.Add(1)
		go m.sweep(interval)
	}
	return m
}

func (m *MemCache) sweep(interval time.Duration) {
	defer m.wg.Done()

	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			m.deleteExpired()
		case <-m.stop:
			return
		}
	}
}

func (m *MemCache) deleteExpired() {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, e := range m.items {
		if e.expired(now) {
			delete(m.items, k)
		}
	}
}

func expiry(ttl time.Duration) time.Time {
	if ttl <= 0 {
		return time.Time{}
	}
	return time.Now().Add(ttl)
}

func (m *MemCache) Get(ctx context.Context, key string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return "", storage.ErrCacheClosed
	}

	e, ok := m.items[key]
	if !ok {
		return "", storage.ErrCacheMiss
	}
	if e.expired(time.Now()) {
		delete(m.items, key)
		return "", storage.ErrCacheMiss
	}
	return e.value, nil
}

func (m *MemCache) Set(ctx context.Context, key, val string, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return storage.ErrCacheClosed
	}

	m.items[key] = entry{value: val, expiresAt: expiry(ttl)}
	return nil
}

func (m *MemCache) Del(ctx context.Context, keys ...string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return storage.ErrCacheClosed
	}

	for _, k := range keys {
		delete(m.items, k)
	}
	return nil
}

func (m *MemCache) Incr(ctx context.Context, key string, delta int64) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, storage.ErrCacheClosed
	}

	var current int64
	if e, ok := m.items[key]; ok && !e.expired(time.Now()) {
		n, err := strconv.ParseInt(e.value, 10, 64)
		if err != nil {
			return 0, err
		}
		current = n
	}

	current += delta
	// A counter created here carries no ttl, matching Redis INCR.
	m.items[key] = entry{value: strconv.FormatInt(current, 10)}
	return current, nil
}

func (m *MemCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return storage.ErrCacheClosed
	}

	e, ok := m.items[key]
	if !ok || e.expired(time.Now()) {
		delete(m.items, key)
		return storage.ErrCacheMiss
	}

	e.expiresAt = expiry(ttl)
	m.items[key] = e
	return nil
}

func (m *MemCache) Close() error {
	m.stopOnce.Do(func() {
		close(m.stop)
	})
	m.wg.Wait()

	m.mu.Lock()
	m.closed = true
	m.items = nil
	m.mu.Unlock()
	return nil
}
