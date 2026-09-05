package runtime

import (
	"testing"

	"github.com/gin-gonic/gin"
)

// GetHandlerFunc is the point of this file: a caller must be able to ask for
// a middleware by key and get back either a working gin.HandlerFunc or a
// clear "not there" answer, never a panic.
func TestGetHandlerFuncReturnsRegisteredHandler(t *testing.T) {
	e := NewConfig()

	called := false
	var h gin.HandlerFunc = func(c *gin.Context) { called = true }
	e.SetMiddleware(JwtTokenCheck, h)

	got, ok := e.GetHandlerFunc(JwtTokenCheck)
	if !ok {
		t.Fatal("GetHandlerFunc reported false for a key that was registered")
	}
	got(nil) //nolint:staticcheck // exercising the stored closure, not gin's dispatch
	if !called {
		t.Error("the handler returned by GetHandlerFunc is not the one that was registered")
	}
}

// An unregistered key is the ordinary "the host has not started yet, or this
// key is unused" case. It must come back as ok=false, not a panic, because
// GetHandlerFunc is usually called from a router initialiser that a caller
// wants to fail loudly and specifically on, not crash generically on.
func TestGetHandlerFuncReportsMissingKey(t *testing.T) {
	e := NewConfig()

	if _, ok := e.GetHandlerFunc(JwtTokenCheck); ok {
		t.Error("GetHandlerFunc reported true for a key that was never registered")
	}
}

// The failure mode this method exists to remove: a value that was registered
// under the right key but is not a gin.HandlerFunc - exactly what
// (*jwt.GinJWTMiddleware).MiddlewareFunc (an unbound method expression, not a
// bound closure) looked like before hosts were required to bind it. A bare
// type assertion at the call site would panic here; GetHandlerFunc must not.
func TestGetHandlerFuncReportsWrongShape(t *testing.T) {
	e := NewConfig()

	e.SetMiddleware(JwtTokenCheck, "not a handler")

	if _, ok := e.GetHandlerFunc(JwtTokenCheck); ok {
		t.Error("GetHandlerFunc reported true for a value that is not a gin.HandlerFunc")
	}
}

// The three keys are the contract, not their values: a host reads and writes
// them by name across independent files (registration in one, lookup in
// another, possibly in a different repository). If a typo ever split them
// into two different constants this would still compile - both are untyped
// string constants - so the values are pinned here as the actual regression
// guard.
func TestMiddlewareKeyValues(t *testing.T) {
	cases := map[string]string{
		"JwtTokenCheck":   JwtTokenCheck,
		"RoleCheck":       RoleCheck,
		"PermissionCheck": PermissionCheck,
	}
	want := map[string]string{
		"JwtTokenCheck":   "JwtToken",
		"RoleCheck":       "AuthCheckRole",
		"PermissionCheck": "PermissionAction",
	}
	for name, got := range cases {
		if got != want[name] {
			t.Errorf("%s = %q, want %q", name, got, want[name])
		}
	}
}

// *Application is the only implementation of Runtime today (docs/contract.md
// section 8), but adding an interface method is still source-breaking for
// anyone with their own. This is the compile-time proof that *Application
// still satisfies Runtime after GetHandlerFunc was added to it.
var _ Runtime = (*Application)(nil)
