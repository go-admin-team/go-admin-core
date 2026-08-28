package cache

import (
	"context"
	"testing"
	"time"
)

// Allocation counts are the half of a performance regression that can be
// asserted. Wall-clock numbers move with whatever else the machine is doing,
// so a CI gate built on them either has a threshold loose enough to miss real
// regressions or tight enough to fail at random. Allocations per operation are
// deterministic: the same code path allocates the same number of times on every
// machine, so a change in the count is a change in the code.
//
// The budgets below are the measured counts with a little headroom. Raising one
// is a deliberate act - it should come with a reason, because these paths run
// on every request.

func assertAllocs(t *testing.T, name string, budget float64, fn func()) {
	t.Helper()
	// Warm the path once: first-call lazy initialisation is not what the
	// budget is about.
	fn()
	got := testing.AllocsPerRun(100, fn)
	if got > budget {
		t.Errorf("%s allocates %.0f times per call, budget is %.0f", name, got, budget)
	}
}

func TestCacheAllocationBudget(t *testing.T) {
	ctx := context.Background()

	t.Run("MemCache", func(t *testing.T) {
		m := NewMemCacheWithSweep(0)
		defer func() { _ = m.Close() }()
		if err := m.Set(ctx, "k", "v", time.Hour); err != nil {
			t.Fatal(err)
		}

		// A hit returns a string already in the map; nothing needs to be built.
		assertAllocs(t, "MemCache.Get(hit)", 0, func() {
			_, _ = m.Get(ctx, "k")
		})
		// A miss must not allocate either - an unauthenticated endpoint can
		// drive misses at will.
		assertAllocs(t, "MemCache.Get(miss)", 0, func() {
			_, _ = m.Get(ctx, "absent")
		})
		assertAllocs(t, "MemCache.Set", 2, func() {
			_ = m.Set(ctx, "k", "v", time.Hour)
		})
	})

	t.Run("Memory", func(t *testing.T) {
		m := NewMemoryWithCleanupInterval(0)
		defer func() { _ = m.Close() }()
		if err := m.Set("k", "v", 3600); err != nil {
			t.Fatal(err)
		}

		assertAllocs(t, "Memory.Get(hit)", 0, func() {
			_, _ = m.Get("k")
		})
	})
}
