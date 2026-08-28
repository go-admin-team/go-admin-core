package search

import "testing"

// ResolveSearchQuery runs on every list request. The budget below is what an
// unfiltered list page costs - the common case, since a list opens before
// anyone types a filter.
//
// It was 51 allocations, because the tag of every field was parsed before the
// field was checked for being zero, and an unfiltered request has nothing but
// zero fields. Moving the check ahead of the parse dropped it to one. This
// pins that: a change that puts the parse back in front fails here rather than
// quietly costing every list request fifty allocations again.
func TestResolveAllocationBudget(t *testing.T) {
	q := benchSearchDTO{BenchPagination: BenchPagination{PageIndex: 1, PageSize: 10}}
	cond := newCondition()

	// Warm once; the first call through a reflect.Type populates caches inside
	// the runtime that later calls reuse.
	ResolveSearchQuery(Mysql, q, cond)

	got := testing.AllocsPerRun(100, func() {
		ResolveSearchQuery(Mysql, q, cond)
	})
	const budget = 2
	if got > budget {
		t.Errorf("an unfiltered search allocates %.0f times, budget is %d; "+
			"the most likely cause is parsing tags before checking for zero values", got, budget)
	}
}
