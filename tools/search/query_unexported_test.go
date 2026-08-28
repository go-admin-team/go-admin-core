package search

import "testing"

// unexportedDTO carries a field the author did not export. Nothing about it is
// exotic - a cached value, a request-scoped helper - but reflection cannot read
// it, and Interface panics rather than returning an error.
type unexportedDTO struct {
	Username string `search:"type:contains;column:username;table:sys_user"`

	// No search tag, so the resolver used to recurse into it and panic with
	// "cannot return value obtained from unexported field or method",
	// taking down the whole list request.
	internal struct{ Cached string }

	// Tagged but unexported: the value can never be read either.
	secret string `search:"type:exact;column:secret"`
}

func TestUnexportedFieldsAreSkipped(t *testing.T) {
	q := unexportedDTO{Username: "admin"}
	q.internal.Cached = "x"
	q.secret = "y"

	cond := &GormCondition{GormPublic: GormPublic{}, Join: make([]*GormJoin, 0)}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("an unexported field on a search DTO panicked the resolver: %v", r)
		}
	}()

	ResolveSearchQuery(Mysql, q, cond)

	// The exported field still has to work; skipping must not become ignoring.
	if len(cond.Where) == 0 {
		t.Error("the exported, tagged field produced no condition")
	}
}
