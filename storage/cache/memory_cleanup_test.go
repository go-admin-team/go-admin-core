package cache

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

// Expired entries must be reclaimed by the background sweeper.
//
// Memory used to expire entries only lazily on Get, so a key written and never
// read again stayed in the sync.Map forever and a long-running process only
// grew (issue #31).

func countItems(m *Memory) int {
	n := 0
	m.items.Range(func(_, _ interface{}) bool {
		n++
		return true
	})
	return n
}

// storeExpired writes an already-expired entry directly. Set takes whole
// seconds, so going through it would add a second of waiting to every test.
func storeExpired(m *Memory, key string) {
	_ = m.setItem(key, &item{Value: "v", Expired: time.Now().Add(-time.Millisecond)})
}

// waitFor polls instead of sleeping a fixed duration, which is flaky on a
// loaded machine.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(2 * time.Millisecond)
	}
	return cond()
}

// An entry that expired and was never read must still be reclaimed.
func TestExpiredItemsAreCollected(t *testing.T) {
	m := NewMemoryWithCleanupInterval(10 * time.Millisecond)
	defer m.Close()

	for i := 0; i < 3; i++ {
		storeExpired(m, fmt.Sprintf("k%d", i))
	}
	if countItems(m) == 0 {
		t.Fatal("no entries after writing; the test premise does not hold")
	}

	if !waitFor(t, time.Second, func() bool { return countItems(m) == 0 }) {
		t.Errorf("expired entries were not reclaimed, %d still resident: keys "+
			"never read again would occupy memory forever", countItems(m))
	}
}

// Live entries must survive the sweep.
func TestUnexpiredItemsSurviveCleanup(t *testing.T) {
	m := NewMemoryWithCleanupInterval(10 * time.Millisecond)
	defer m.Close()

	if err := m.Set("keep", "value", 60); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	storeExpired(m, "drop")

	// Wait until a sweep has demonstrably run, signalled by the expired entry
	// being gone.
	if !waitFor(t, time.Second, func() bool { return countItems(m) == 1 }) {
		t.Fatalf("the sweep did not run as expected, %d entries remain", countItems(m))
	}

	v, err := m.Get("keep")
	if err != nil {
		t.Fatalf("a live entry was swept away: %v", err)
	}
	if v != "value" {
		t.Errorf("wrong value: got %q, want %q", v, "value")
	}
}

// Close must stop the goroutine and be safe to call twice. Its internal
// wg.Wait already guarantees the goroutine has exited, so no sleep is needed.
func TestCloseStopsJanitor(t *testing.T) {
	before := runtime.NumGoroutine()

	m := NewMemoryWithCleanupInterval(10 * time.Millisecond)
	if err := m.Close(); err != nil {
		t.Fatalf("Close returned an error: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("a second Close returned an error: %v", err)
	}

	if after := runtime.NumGoroutine(); after > before {
		t.Errorf("goroutine count did not drop after Close (%d -> %d)",
			before, after)
	}
}

// A zero interval falls back to purely lazy expiry and starts no goroutine.
func TestZeroIntervalDisablesJanitor(t *testing.T) {
	before := runtime.NumGoroutine()

	m := NewMemoryWithCleanupInterval(0)
	defer m.Close()

	storeExpired(m, "stale")

	// Ample opportunity for a sweeper to run had one been started.
	time.Sleep(50 * time.Millisecond)

	if after := runtime.NumGoroutine(); after > before {
		t.Errorf("a sweeper was started despite a zero interval (%d -> %d)", before, after)
	}
	if n := countItems(m); n != 1 {
		t.Errorf("no sweep expected with a zero interval; want 1 entry, got %d", n)
	}
}
