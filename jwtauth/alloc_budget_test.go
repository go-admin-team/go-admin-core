package jwtauth

import "testing"

// Parsing a token is the fixed cost of every authenticated request. The budget
// is generous because the work is genuinely allocation-heavy - HMAC
// verification plus JSON decoding into a claim map - but it still catches a
// change that makes it materially worse, which is what a gate is for.
//
// Allocation counts are deterministic; see the note in
// storage/cache/alloc_budget_test.go for why the timing figures are not gated.
func TestParseTokenAllocationBudget(t *testing.T) {
	mw := newBenchMiddlewareForTest(t)
	token, _, err := mw.TokenGenerator(nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := mw.ParseTokenString(token); err != nil {
		t.Fatal(err)
	}

	got := testing.AllocsPerRun(50, func() {
		_, _ = mw.ParseTokenString(token)
	})
	const budget = 90
	if got > budget {
		t.Errorf("ParseTokenString allocates %.0f times, budget is %d", got, budget)
	}
}
