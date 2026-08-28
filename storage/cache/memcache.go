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

// shardCount is how many independently locked maps the keyspace is split
// across. It must be a power of two so the index is a mask rather than a
// modulo.
//
// One mutex over one map is simpler, and that is what this was. It cost two
// things. Every operation contended on the same lock, which is why reads were
// several times slower than the sync.Map implementation next door. And the
// periodic sweep walks the whole map under that lock, so the cache stalled for
// the length of the walk: at a million entries, reads that normally take 33µs
// took 16ms - the sweep duration, exactly. Sharding bounds both by the size of
// one shard.
const shardCount = 32

// memShard is one independently locked partition. Incr still needs an atomic
// read-modify-write and still gets one: a key always maps to the same shard, so
// the operations on it remain mutually exclusive.
type memShard struct {
	mu     sync.Mutex
	items  map[string]entry
	closed bool
}

// MemCache is an in-process storage.Cache.
type MemCache struct {
	shards [shardCount]memShard

	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// shard picks the partition for a key with FNV-1a, which is short enough to
// inline and spreads the uuid-shaped keys this cache mostly holds.
func (m *MemCache) shard(key string) *memShard {
	const (
		offset64 = uint32(2166136261)
		prime64  = uint32(16777619)
	)
	h := offset64
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= prime64
	}
	return &m.shards[h&(shardCount-1)]
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
	m := &MemCache{stop: make(chan struct{})}
	for i := range m.shards {
		m.shards[i].items = make(map[string]entry)
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

// deleteExpired reclaims entries nobody reads any more. Lazy expiry handles
// everything that is read again; this exists for the rest, which would
// otherwise be held for the life of the process.
//
// It takes one shard at a time. The whole map is still walked, but no single
// acquisition covers more than a shard of it, so the stall any concurrent
// operation can see is bounded by shard size rather than cache size.
func (m *MemCache) deleteExpired() {
	now := time.Now()
	for i := range m.shards {
		s := &m.shards[i]
		s.mu.Lock()
		if !s.closed {
			for k, e := range s.items {
				if e.expired(now) {
					delete(s.items, k)
				}
			}
		}
		s.mu.Unlock()
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

	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", storage.ErrCacheClosed
	}

	e, ok := s.items[key]
	if !ok {
		return "", storage.ErrCacheMiss
	}
	if e.expired(time.Now()) {
		delete(s.items, key)
		return "", storage.ErrCacheMiss
	}
	return e.value, nil
}

func (m *MemCache) Set(ctx context.Context, key, val string, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return storage.ErrCacheClosed
	}

	s.items[key] = entry{value: val, expiresAt: expiry(ttl)}
	return nil
}

func (m *MemCache) Del(ctx context.Context, keys ...string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Keys are locked one shard at a time rather than all at once; the
	// contract makes no atomicity promise across keys.
	for _, k := range keys {
		s := m.shard(k)
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return storage.ErrCacheClosed
		}
		delete(s.items, k)
		s.mu.Unlock()
	}
	return nil
}

func (m *MemCache) Incr(ctx context.Context, key string, delta int64) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, storage.ErrCacheClosed
	}

	var current int64
	if e, ok := s.items[key]; ok && !e.expired(time.Now()) {
		n, err := strconv.ParseInt(e.value, 10, 64)
		if err != nil {
			return 0, err
		}
		current = n
	}

	current += delta
	// A counter created here carries no ttl, matching Redis INCR.
	s.items[key] = entry{value: strconv.FormatInt(current, 10)}
	return current, nil
}

func (m *MemCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return storage.ErrCacheClosed
	}

	e, ok := s.items[key]
	if !ok || e.expired(time.Now()) {
		delete(s.items, key)
		return storage.ErrCacheMiss
	}

	e.expiresAt = expiry(ttl)
	s.items[key] = e
	return nil
}

func (m *MemCache) Close() error {
	m.stopOnce.Do(func() {
		close(m.stop)
	})
	m.wg.Wait()

	for i := range m.shards {
		s := &m.shards[i]
		s.mu.Lock()
		s.closed = true
		s.items = nil
		s.mu.Unlock()
	}
	return nil
}
