package cache

import (
	"context"
	"os"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

// Peak throughput says nothing about whether a process survives a week. These
// run a steady mixed load and watch the things that only go wrong slowly:
// goroutines that are never reaped, and a heap that never comes back down.
//
// They are skipped under -short, which is what CI runs today. Give them time
// with:
//
//	GOADMIN_SOAK=2m go test -run Soak ./storage/cache/
const soakEnv = "GOADMIN_SOAK"

func soakDuration(t *testing.T) time.Duration {
	t.Helper()
	if testing.Short() {
		t.Skipf("soak test; skipped under -short (set %s to run it longer)", soakEnv)
	}
	if v := os.Getenv(soakEnv); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			t.Fatalf("%s=%q: %v", soakEnv, v, err)
		}
		return d
	}
	return 15 * time.Second
}

// heapInUse reports the live heap after giving the collector two chances to
// run. One GC is not enough: the first can leave objects finalised but not yet
// freed, which reads as growth that is not there.
func heapInUse() uint64 {
	runtime.GC()
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.HeapInuse
}

// TestMemCacheSoak keeps a bounded working set under continuous mixed traffic
// with short TTLs. The heap must reach a plateau: entries expire and are
// reclaimed at roughly the rate they arrive, so a heap that keeps climbing
// means expiry is not keeping up with writes.
func TestMemCacheSoak(t *testing.T) {
	duration := soakDuration(t)

	ctx := context.Background()
	m := NewMemCacheWithSweep(200 * time.Millisecond)
	defer func() { _ = m.Close() }()

	goroutinesBefore := runtime.NumGoroutine()

	stop := make(chan struct{})
	var wg sync.WaitGroup

	const workers = 8
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			i := 0
			for {
				select {
				case <-stop:
					return
				default:
				}
				// A bounded key space with short TTLs: the steady state is a
				// working set, not unbounded accumulation.
				key := "soak-" + strconv.Itoa(id) + "-" + strconv.Itoa(i%5000)
				_ = m.Set(ctx, key, "value", 300*time.Millisecond)
				_, _ = m.Get(ctx, key)
				if i%10 == 0 {
					_, _ = m.Incr(ctx, "counter-"+strconv.Itoa(id), 1)
				}
				if i%100 == 0 {
					_ = m.Del(ctx, key)
				}
				i++
			}
		}(w)
	}

	// Sample after the working set has filled, so the baseline is the plateau
	// rather than the ramp.
	time.Sleep(duration / 3)
	mid := heapInUse()

	time.Sleep(duration - duration/3)
	close(stop)
	wg.Wait()

	end := heapInUse()

	t.Logf("heap at plateau %.1f MB, at end %.1f MB over %v",
		float64(mid)/(1024*1024), float64(end)/(1024*1024), duration)

	// Doubling from an established plateau is growth, not noise.
	if end > mid*2 {
		t.Errorf("heap grew from %d to %d bytes after the working set had settled; "+
			"expired entries are not being reclaimed", mid, end)
	}

	// Workers are done and the sweeper is the only goroutine the cache owns.
	time.Sleep(100 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > goroutinesBefore+2 {
		t.Errorf("goroutines went from %d to %d", goroutinesBefore, after)
	}
}

// TestMemCacheCloseReleasesEverything is the shutdown path. A server that
// rebuilds its cache on a config reload does this repeatedly, and a sweeper
// that outlives its cache accumulates one goroutine per reload.
func TestMemCacheCloseReleasesEverything(t *testing.T) {
	ctx := context.Background()
	before := runtime.NumGoroutine()

	for i := 0; i < 50; i++ {
		m := NewMemCacheWithSweep(time.Millisecond)
		for j := 0; j < 100; j++ {
			if err := m.Set(ctx, "k"+strconv.Itoa(j), "v", time.Minute); err != nil {
				t.Fatal(err)
			}
		}
		if err := m.Close(); err != nil {
			t.Fatal(err)
		}
	}

	time.Sleep(50 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > before {
		t.Errorf("50 cache lifecycles left %d goroutines behind (%d -> %d)",
			after-before, before, after)
	}
}
