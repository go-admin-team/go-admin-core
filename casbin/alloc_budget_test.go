package mycasbin

import "testing"

// Every authorised request runs one Enforce, and its cost is dominated by what
// the matcher allocates per policy it tests. The builtin keyMatch2 compiles a
// regexp each time - about 98 allocations - so the budget below is what keeps
// the cached matcher wired in: revert the model or the registration and this
// fails, where a benchmark would only look slower on a machine nobody watches.
//
// Allocation counts are deterministic across machines; wall-clock is not.
func TestEnforceAllocationBudget(t *testing.T) {
	// The same fixture the benchmarks use: one role holding all of these, with
	// the matching rule last so Enforce walks every one of them.
	const policies = 50
	e := sameSubjectEnforcer(t, policies, false)

	ok, err := e.Enforce("operator", "/api/v1/dept", "GET")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("setup failed: the request is not allowed")
	}

	// Measured at 419 with the cached matcher and 4863 with the builtin. The
	// budget sits between the two with room for the evaluator to change, and
	// well below the number a regression would produce.
	const budget = 900

	got := testing.AllocsPerRun(50, func() {
		if _, err := e.Enforce("operator", "/api/v1/dept", "GET"); err != nil {
			t.Fatal(err)
		}
	})
	if got > budget {
		t.Errorf("Enforce over %d policies allocates %.0f times, budget is %d\n"+
			"the builtin keyMatch2 costs about 4863 here; check that the matcher still calls %s",
			policies, got, budget, keyMatch2CachedName)
	}
}
