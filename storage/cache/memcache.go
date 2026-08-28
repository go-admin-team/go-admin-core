package cache

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/go-admin-team/go-admin-core/v2/storage"
)

const defaultSweepInterval = time.Minute

// defaultMaxEntries bounds how much an in-process cache can hold.
//
// Without a bound the cache grows until the process dies, and reaching that
// state needs no bug. Any endpoint that writes an entry under a key it will
// never reuse - a one-time token, a per-request nonce - grows the cache at
// whatever rate callers can drive, and an unauthenticated one puts that rate in
// the caller's hands. At roughly 180 bytes an entry, a few thousand writes a
// second is hundreds of megabytes before the first ttl expires.
//
// A million entries is about 180MB: far above what an application legitimately
// caches in one process, far below what it takes to exhaust a machine. The
// point is that a number exists, not that this one is exactly right; pass
// MemCacheOptions.MaxEntries to choose another.
//
// The bound is on this implementation only. Memory, in the same package, backs
// the deprecated AdapterCache and is still unbounded.
const defaultMaxEntries = 1 << 20

// MemCacheOptions configures NewMemCacheWithOptions.
type MemCacheOptions struct {
	// SweepInterval is how often expired entries are collected. Zero or less
	// starts no sweeper, leaving expiry entirely lazy.
	SweepInterval time.Duration

	// MaxEntries caps the number of entries held. Zero means
	// defaultMaxEntries; a negative value means no limit, which is only
	// appropriate when the keyspace is known to be bounded by something else.
	//
	// The cap is enforced per shard, so the effective total is rounded down to
	// a multiple of shardCount.
	MaxEntries int
}

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

	// maxPerShard is the entry budget of one shard, or zero for no limit.
	maxPerShard int

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
	return NewMemCacheWithOptions(MemCacheOptions{SweepInterval: defaultSweepInterval})
}

// NewMemCacheWithSweep sets the sweep interval. Zero or less starts no
// sweeper, leaving expiry entirely lazy.
func NewMemCacheWithSweep(interval time.Duration) *MemCache {
	return NewMemCacheWithOptions(MemCacheOptions{SweepInterval: interval})
}

// NewMemCacheWithOptions builds a cache from an explicit configuration.
func NewMemCacheWithOptions(o MemCacheOptions) *MemCache {
	m := &MemCache{stop: make(chan struct{})}
	for i := range m.shards {
		m.shards[i].items = make(map[string]entry)
	}

	switch {
	case o.MaxEntries < 0:
		m.maxPerShard = 0 // unlimited
	case o.MaxEntries == 0:
		m.maxPerShard = defaultMaxEntries / shardCount
	default:
		// At least one per shard, so a small explicit cap is not rounded to
		// zero and read as "unlimited".
		if per := o.MaxEntries / shardCount; per > 0 {
			m.maxPerShard = per
		} else {
			m.maxPerShard = 1
		}
	}

	if o.SweepInterval > 0 {
		m.wg.Add(1)
		go m.sweep(o.SweepInterval)
	}
	return m
}

// evictionSample bounds how many entries one eviction may examine.
//
// The cap is what keeps eviction off the critical path. Once a shard is at its
// limit, freeing one slot lets the caller insert one - which fills it again, so
// the next write of a new key evicts too. Eviction is therefore per-write in
// the steady state, not occasional, and a scan of the whole shard here would
// cost every write what the unsharded sweep used to cost once a minute. The
// workload that reaches the limit at all is precisely the one that writes a new
// key every time.
const evictionSample = 64

// evictIfFull frees a slot when the shard is at its limit and the pending write
// would add a key rather than replace one. The caller holds the shard lock.
//
// The key lookup happens only once the shard is known to be full, so a cache
// that never fills pays a single length comparison.
func (s *memShard) evictIfFull(limit int, key string) {
	if limit <= 0 || len(s.items) < limit {
		return
	}
	if _, replacing := s.items[key]; replacing {
		return
	}
	s.makeRoom(limit)
}

// makeRoom frees a slot in a shard that has reached its budget. The caller
// holds the shard lock.
//
// Expired entries go first: they are owed to nobody. The search for one stops
// after evictionSample entries rather than scanning the shard to prove there is
// none - the sweep reclaims the rest on its own schedule, and this path cannot
// afford to look. Failing that it drops an entry chosen by map iteration order,
// which Go leaves unspecified and is therefore effectively arbitrary.
//
// Arbitrary rather than least-recently-used is deliberate. Tracking recency
// means a linked list updated on every read, and reads are the operation this
// cache exists to make cheap; an approximate policy on a cache whose contract
// already allows any entry to vanish is the better trade.
func (s *memShard) makeRoom(limit int) {
	now := time.Now()

	scanned := 0
	for k, e := range s.items {
		if e.expired(now) {
			delete(s.items, k)
			if len(s.items) < limit {
				return
			}
		}
		scanned++
		if scanned >= evictionSample {
			break
		}
	}

	// Nothing reclaimable in the sample. One live entry has to go; the loop
	// exits on the first iteration, since the shard holds exactly limit.
	for k := range s.items {
		delete(s.items, k)
		if len(s.items) < limit {
			return
		}
	}
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

	s.evictIfFull(m.maxPerShard, key)
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
	s.evictIfFull(m.maxPerShard, key)
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
