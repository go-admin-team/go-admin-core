package api

import (
	"slices"
	"testing"

	"github.com/gin-gonic/gin/binding"
)

// Issue #72: binding tags on anonymously embedded structs must be resolved.
//
// Generated search DTOs use this shape, grouping ordering fields into an
// embedded struct. Fixtures are reused from binding_test.go rather than
// duplicating an identically shaped search DTO in the same package.

// All binding tags come from embedding; the outer struct declares no fields of
// its own. This is a real shape: a list endpoint with paging and ordering but no
// filters.
type searchOnlyEmbedded struct {
	Pagination `search:"-"`
	SysUserOrder
}

// Embedded pointer, the *Order form called out in the issue.
type searchEmbeddedPtr struct {
	*SysUserOrder
}

// A struct whose tags come only from embedding must still resolve to Form.
func TestResolveEmbeddedStructOnly(t *testing.T) {
	bs := constructor.GetBindingForGin(&searchOnlyEmbedded{})

	if !slices.Contains(bs, binding.Form) {
		t.Errorf("a search DTO whose tags come only from embedding resolved to "+
			"%v: pageIndex, pageSize and the ordering parameters would not bind", bs)
	}
}

// An embedded pointer must be resolved too.
func TestResolveEmbeddedPointer(t *testing.T) {
	bs := constructor.GetBindingForGin(&searchEmbeddedPtr{})

	if !slices.Contains(bs, binding.Form) {
		t.Errorf("form tags in an embedded pointer struct were not resolved: %v", bs)
	}
}

// The full generated shape: form tags on the outer struct plus embedded paging
// and ordering. The outer tags alone already select Form, so this mainly guards
// against the recursion regressing existing behaviour.
func TestResolveGeneratedSearchDTO(t *testing.T) {
	bs := constructor.GetBindingForGin(&SysUserSearch{})

	if !slices.Contains(bs, binding.Form) {
		t.Errorf("a generated-shape search DTO did not resolve to Form: %v", bs)
	}
	if slices.Contains(bs, nil) {
		t.Errorf("this DTO has no uri tag, so no uri binder is expected: %v", bs)
	}
}
