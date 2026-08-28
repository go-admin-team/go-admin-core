package pkg

import (
	"testing"

	"gorm.io/gorm"
)

// Both helpers run on every request. Allocation counts are deterministic, so
// these are gates rather than measurements - see the note in
// storage/cache/alloc_budget_test.go.
func TestPkgAllocationBudget(t *testing.T) {
	c := benchContext(true)
	c.Set("db", new(gorm.DB))

	// A type assertion off the context map; nothing is constructed.
	if got := testing.AllocsPerRun(100, func() { _, _ = GetOrm(c) }); got > 0 {
		t.Errorf("GetOrm allocates %.0f times, budget is 0", got)
	}

	// The failure path is on every request of a misconfigured route, so it is
	// budgeted too - a fresh error per call is a fresh allocation per call.
	missing := benchContext(true)
	if got := testing.AllocsPerRun(100, func() { _, _ = GetOrm(missing) }); got > 0 {
		t.Errorf("GetOrm on a request without a db allocates %.0f times, budget is 0", got)
	}

	// When a gateway already supplied the correlation id this is a header read.
	if got := testing.AllocsPerRun(100, func() { _ = GenerateMsgIDFromContext(c) }); got > 0 {
		t.Errorf("GenerateMsgIDFromContext with an existing id allocates %.0f times, budget is 0", got)
	}
}
