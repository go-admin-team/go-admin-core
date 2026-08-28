package cache

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

// The in-memory cache is what a single-instance deployment runs on: leaving the
// redis section unset in settings.yml selects it. Everything here therefore
// exercises it the way a live server does - every operation from many
// goroutines at once - rather than one call at a time.
//
// Run the correctness tests under -race. The throughput benchmarks are meant to
// be read as a ceiling for one process, so run those without it.

const (
	concurrentWriters = 50
	opsPerWriter      = 200
)

// TestMemoryIncreaseIsAtomic pins the counter contract. Increase used to take
// the read lock, which does not exclude other writers, so concurrent callers
// read the same value and wrote back the same result. The count came out around
// 10% short and the race detector flagged the write.
func TestMemoryIncreaseIsAtomic(t *testing.T) {
	m := NewMemoryWithCleanupInterval(0)
	defer func() { _ = m.Close() }()

	if err := m.Set("n", 0, 60); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(concurrentWriters)
	for i := 0; i < concurrentWriters; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerWriter; j++ {
				if err := m.Increase("n"); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()

	got, err := m.Get("n")
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(got)
	if err != nil {
		t.Fatal(err)
	}
	if want := concurrentWriters * opsPerWriter; n != want {
		t.Errorf("lost %d of %d increments, counter reports %d", want-n, want, n)
	}
}

// TestMemoryDecreaseIsAtomic covers the other direction, which shares the
// implementation but not the test.
func TestMemoryDecreaseIsAtomic(t *testing.T) {
	m := NewMemoryWithCleanupInterval(0)
	defer func() { _ = m.Close() }()

	start := concurrentWriters * opsPerWriter
	if err := m.Set("n", start, 60); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(concurrentWriters)
	for i := 0; i < concurrentWriters; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerWriter; j++ {
				if err := m.Decrease("n"); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()

	got, err := m.Get("n")
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(got)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("counter should be back to 0, got %d", n)
	}
}

// TestMemoryExpireDoesNotRaceWithReaders covers the second half of the same
// defect: Expire reached through the pointer it had just read to reset the
// deadline, and that pointer is published in the map where readers hold it.
func TestMemoryExpireDoesNotRaceWithReaders(t *testing.T) {
	m := NewMemoryWithCleanupInterval(0)
	defer func() { _ = m.Close() }()

	if err := m.Set("k", "v", 60); err != nil {
		t.Fatal(err)
	}

	stop := make(chan struct{})
	var readers sync.WaitGroup

	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			select {
			case <-stop:
				return
			default:
				if _, err := m.Get("k"); err != nil {
					t.Error(err)
					return
				}
			}
		}
	}()

	for i := 0; i < 2000; i++ {
		if err := m.Expire("k", time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	close(stop)
	readers.Wait()
}

// TestMemoryMixedTrafficIsRaceFree runs the four operations a request path
// actually mixes. It asserts nothing beyond "no race and no error", which is
// what -race is here to check.
func TestMemoryMixedTrafficIsRaceFree(t *testing.T) {
	m := NewMemoryWithCleanupInterval(10 * time.Millisecond)
	defer func() { _ = m.Close() }()

	if err := m.Set("counter", 0, 60); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < concurrentWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", id)
			for j := 0; j < opsPerWriter; j++ {
				if err := m.Set(key, j, 60); err != nil {
					t.Error(err)
					return
				}
				if _, err := m.Get(key); err != nil {
					t.Error(err)
					return
				}
				if err := m.Increase("counter"); err != nil {
					t.Error(err)
					return
				}
				if err := m.Expire(key, time.Minute); err != nil {
					t.Error(err)
					return
				}
			}
			if err := m.Del(key); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
}

// --- throughput ---------------------------------------------------------
//
// RunParallel spreads the work over GOMAXPROCS goroutines, so these report what
// one process sustains with every core pushing, not a single-threaded figure.

func BenchmarkMemoryGet(b *testing.B) {
	m := NewMemoryWithCleanupInterval(0)
	defer func() { _ = m.Close() }()
	if err := m.Set("k", "v", 3600); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := m.Get("k"); err != nil {
				b.Error(err)
				return
			}
		}
	})
}

func BenchmarkMemorySet(b *testing.B) {
	m := NewMemoryWithCleanupInterval(0)
	defer func() { _ = m.Close() }()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Distinct keys per goroutine, so this measures the store rather
			// than contention on one map bucket.
			if err := m.Set("k"+strconv.Itoa(i%64), "v", 3600); err != nil {
				b.Error(err)
				return
			}
			i++
		}
	})
}

// BenchmarkMemoryIncrease is the serialised path: every caller takes the same
// write lock, so this is the one that does not scale with cores.
func BenchmarkMemoryIncrease(b *testing.B) {
	m := NewMemoryWithCleanupInterval(0)
	defer func() { _ = m.Close() }()
	if err := m.Set("n", 0, 3600); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := m.Increase("n"); err != nil {
				b.Error(err)
				return
			}
		}
	})
}

// --- the same three against the current contract ------------------------
//
// MemCache is what Cache.Open returns. Setup, which is what most call sites
// still go through, returns Memory above. Keeping both here makes the cost of
// staying on the deprecated one visible.

// BenchmarkMemCacheGet spreads reads over many keys, which is what a cache
// holding sessions, captchas or per-user data actually sees.
func BenchmarkMemCacheGet(b *testing.B) {
	ctx := context.Background()
	m := NewMemCacheWithSweep(0)
	defer func() { _ = m.Close() }()
	for i := 0; i < 1024; i++ {
		if err := m.Set(ctx, "k"+strconv.Itoa(i), "v", time.Hour); err != nil {
			b.Fatal(err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if _, err := m.Get(ctx, "k"+strconv.Itoa(i%1024)); err != nil {
				b.Error(err)
				return
			}
			i++
		}
	})
}

// BenchmarkMemCacheGetHotKey is the worst case: every goroutine reads the same
// key, so they all land on one shard and sharding buys nothing. Keeping it
// alongside the spread version stops the spread number from being read as a
// promise about hot keys.
func BenchmarkMemCacheGetHotKey(b *testing.B) {
	ctx := context.Background()
	m := NewMemCacheWithSweep(0)
	defer func() { _ = m.Close() }()
	if err := m.Set(ctx, "k", "v", time.Hour); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := m.Get(ctx, "k"); err != nil {
				b.Error(err)
				return
			}
		}
	})
}

func BenchmarkMemCacheSet(b *testing.B) {
	ctx := context.Background()
	m := NewMemCacheWithSweep(0)
	defer func() { _ = m.Close() }()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if err := m.Set(ctx, "k"+strconv.Itoa(i%64), "v", time.Hour); err != nil {
				b.Error(err)
				return
			}
			i++
		}
	})
}

// BenchmarkMemCacheIncr counts on one key, which is what a rate limiter or a
// failed-login counter does. It is a hot key by nature, so it does not scale
// with shards - the serialisation is the point of the operation.
func BenchmarkMemCacheIncr(b *testing.B) {
	ctx := context.Background()
	m := NewMemCacheWithSweep(0)
	defer func() { _ = m.Close() }()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := m.Incr(ctx, "n", 1); err != nil {
				b.Error(err)
				return
			}
		}
	})
}

// BenchmarkMemCacheIncrSpread counts on many keys - per-user or per-IP
// counters, which is the shape that should scale.
func BenchmarkMemCacheIncrSpread(b *testing.B) {
	ctx := context.Background()
	m := NewMemCacheWithSweep(0)
	defer func() { _ = m.Close() }()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if _, err := m.Incr(ctx, "n"+strconv.Itoa(i%1024), 1); err != nil {
				b.Error(err)
				return
			}
			i++
		}
	})
}
