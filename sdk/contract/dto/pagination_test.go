package dto

import "testing"

func TestPaginationDefaults(t *testing.T) {
	var p Pagination
	if got := p.GetPageIndex(); got != 1 {
		t.Fatalf("GetPageIndex() on zero value = %d, want 1", got)
	}
	if got := p.GetPageSize(); got != 10 {
		t.Fatalf("GetPageSize() on zero value = %d, want 10", got)
	}

	p2 := Pagination{PageIndex: 3, PageSize: 50}
	if got := p2.GetPageIndex(); got != 3 {
		t.Fatalf("GetPageIndex() = %d, want 3", got)
	}
	if got := p2.GetPageSize(); got != 50 {
		t.Fatalf("GetPageSize() = %d, want 50", got)
	}
}
