package cache

import (
	"context"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// A cache that stalls while it tidies up is a cache that adds a spike to the
// latency graph once a minute, and the spike grows with the data. These
// measure the readers rather than the sweep, because the reader is what the
// user experiences.

// countEntries totals the entries across every shard. Tests that assert on
// capacity or reclamation need the whole picture, and a shard's map is only
// safe to read under its own lock.
func countEntries(m *MemCache) int {
	n := 0
	for i := range m.shards {
		s := &m.shards[i]
		s.mu.Lock()
		n += len(s.items)
		s.mu.Unlock()
	}
	return n
}

// sweepLatencyEntries is large enough for a full walk to be clearly measurable
// and small enough to keep the test quick.
const sweepLatencyEntries = 500000

// TestSweepDoesNotStallReaders bounds what a concurrent reader can be made to
// wait for. A single mutex over the whole map put this at the full duration of
// the walk - a reader normally served in tens of microseconds waited the entire
// sweep. Sharding bounds it by one shard.
func TestSweepDoesNotStallReaders(t *testing.T) {
	if testing.Short() {
		t.Skip("fills half a million entries; skipped under -short")
	}

	ctx := context.Background()
	m := NewMemCacheWithSweep(0)
	defer func() { _ = m.Close() }()

	for i := 0; i < sweepLatencyEntries; i++ {
		if err := m.Set(ctx, "k"+strconv.Itoa(i), "v", time.Hour); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Set(ctx, "live", "v", time.Hour); err != nil {
		t.Fatal(err)
	}

	var worst atomic.Int64
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			t0 := time.Now()
			if _, err := m.Get(ctx, "live"); err != nil {
				t.Error(err)
				return
			}
			d := time.Since(t0).Nanoseconds()
			for {
				cur := worst.Load()
				if d <= cur || worst.CompareAndSwap(cur, d) {
					break
				}
			}
		}
	}()

	// Let the reader warm up, then reset so the measurement covers the sweep.
	time.Sleep(100 * time.Millisecond)
	worst.Store(0)

	start := time.Now()
	m.deleteExpired()
	sweep := time.Since(start)

	close(stop)
	<-done

	stalled := time.Duration(worst.Load())
	t.Logf("sweep of %d entries took %v; worst concurrent Get %v",
		sweepLatencyEntries, sweep.Round(time.Microsecond), stalled.Round(time.Microsecond))

	// The reader must not be held for the whole walk. Half is a generous bound
	// that still fails loudly if the sweep ever goes back to holding one lock
	// across the entire map - with 32 shards the real figure is far below it.
	if stalled > sweep/2 {
		t.Errorf("a reader waited %v during a %v sweep; the sweep is holding a lock "+
			"across more of the map than one shard", stalled, sweep)
	}
}

// TestSweepReclaimsEveryShard guards the other half: bounding the stall must
// not leave entries behind. Every shard has to be visited.
func TestSweepReclaimsEveryShard(t *testing.T) {
	ctx := context.Background()
	m := NewMemCacheWithSweep(0)
	defer func() { _ = m.Close() }()

	const n = 20000
	for i := 0; i < n; i++ {
		if err := m.Set(ctx, "k"+strconv.Itoa(i), "v", time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(20 * time.Millisecond)

	m.deleteExpired()

	if remaining := countEntries(m); remaining != 0 {
		t.Errorf("%d of %d expired entries survived the sweep", remaining, n)
	}
}
