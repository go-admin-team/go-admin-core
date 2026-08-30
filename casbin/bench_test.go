package mycasbin

import (
	"strconv"
	"strings"
	"testing"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
)

// Every request to a protected route runs one Enforce. The matcher compares the
// subject and then calls keyMatch2 and keyMatch on the path, per policy, so the
// cost grows with the number of rules - and the rule count grows with the
// application, one row per API the roles can reach.
//
// go-admin uses a SyncedEnforcer, whose Enforce takes a read lock, so these run
// in parallel as well: the question is not only how long one check takes but
// whether checks get in each other's way.

// benchEnforcer builds an enforcer with n decoy policies plus one that matches,
// placed last so the matcher has to walk everything first. That is the honest
// case to measure: a hit found immediately would report the best position in
// the table rather than a typical one.
func benchEnforcer(b *testing.B, n int) *casbin.SyncedEnforcer {
	b.Helper()

	e, err := newEnforcer(nil)
	if err != nil {
		b.Fatal(err)
	}

	rules := make([][]string, 0, n+1)
	for i := 0; i < n; i++ {
		rules = append(rules, []string{
			"role" + strconv.Itoa(i%16),
			"/api/v1/resource" + strconv.Itoa(i) + "/:id",
			"GET",
		})
	}
	rules = append(rules, []string{"admin", "/api/v1/dept", "GET"})

	if _, err := e.AddPolicies(rules); err != nil {
		b.Fatal(err)
	}
	return e
}

// runEnforce is the timing loop every benchmark here shares. SyncedEnforcer
// takes a read lock, so this runs in parallel: the question is not only how
// long one check takes but whether checks get in each other's way.
func runEnforce(b *testing.B, e *casbin.SyncedEnforcer, sub, obj, act string, want bool) {
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ok, err := e.Enforce(sub, obj, act)
			if err != nil {
				b.Error(err)
				return
			}
			if ok != want {
				b.Errorf("Enforce = %v, want %v", ok, want)
				return
			}
		}
	})
}

func benchEnforce(b *testing.B, n int, sub, obj, act string, want bool) {
	runEnforce(b, benchEnforcer(b, n), sub, obj, act, want)
}

// Allowed: the request matches, but only after every decoy has been tried.
func BenchmarkEnforceAllow10(b *testing.B) { benchEnforce(b, 10, "admin", "/api/v1/dept", "GET", true) }
func BenchmarkEnforceAllow100(b *testing.B) {
	benchEnforce(b, 100, "admin", "/api/v1/dept", "GET", true)
}
func BenchmarkEnforceAllow1000(b *testing.B) {
	benchEnforce(b, 1000, "admin", "/api/v1/dept", "GET", true)
}
func BenchmarkEnforceAllow5000(b *testing.B) {
	benchEnforce(b, 5000, "admin", "/api/v1/dept", "GET", true)
}

// Denied: nothing matches, so the whole table is walked every time. This is
// what an unauthorised request costs, and it is the worst case by construction.
func BenchmarkEnforceDeny1000(b *testing.B) {
	benchEnforce(b, 1000, "nobody", "/api/v1/secret", "DELETE", false)
}

// The benchmarks above give every decoy a different subject, so the matcher
// short-circuits on r.sub == p.sub and only one policy reaches keyMatch2. Real
// policy tables are not shaped that way: one role holds all of its own
// permissions, so every one of them reaches the path comparison.
//
// That is what the pair below measures, and why the model calls
// keyMatch2Cached rather than casbin's builtin - the builtin compiles a regexp
// per policy tested.
//
// Position matters as much as count: the effect is some(where (p.eft ==
// allow)), so Enforce stops at the first match. A permission near the top of
// the table is cheap however long the table is; one near the bottom, and every
// denied request, pays for the whole walk. The matching rule goes last here
// for that reason.
func sameSubjectEnforcer(tb testing.TB, n int, builtin bool) *casbin.SyncedEnforcer {
	tb.Helper()

	var e *casbin.SyncedEnforcer
	var err error
	if builtin {
		// The comparison arm: casbin's own keyMatch2, reached by naming it in
		// the matcher, since AddFunction cannot replace a builtin.
		var m model.Model
		m, err = model.NewModelFromString(
			strings.Replace(text, keyMatch2CachedName+"(", "keyMatch2(", 1))
		if err != nil {
			tb.Fatal(err)
		}
		e, err = casbin.NewSyncedEnforcer(m)
	} else {
		e, err = newEnforcer(nil)
	}
	if err != nil {
		tb.Fatal(err)
	}

	rules := make([][]string, 0, n+1)
	for i := 0; i < n; i++ {
		rules = append(rules, []string{"operator", "/api/v1/resource" + strconv.Itoa(i) + "/:id", "GET"})
	}
	rules = append(rules, []string{"operator", "/api/v1/dept", "GET"})
	if _, err := e.AddPolicies(rules); err != nil {
		tb.Fatal(err)
	}
	return e
}

func benchSameSubject(b *testing.B, n int, builtin bool) {
	runEnforce(b, sameSubjectEnforcer(b, n, builtin), "operator", "/api/v1/dept", "GET", true)
}

func BenchmarkOwnPermissionsBuiltin10(b *testing.B)  { benchSameSubject(b, 10, true) }
func BenchmarkOwnPermissionsBuiltin50(b *testing.B)  { benchSameSubject(b, 50, true) }
func BenchmarkOwnPermissionsBuiltin200(b *testing.B) { benchSameSubject(b, 200, true) }

func BenchmarkOwnPermissionsCached10(b *testing.B)  { benchSameSubject(b, 10, false) }
func BenchmarkOwnPermissionsCached50(b *testing.B)  { benchSameSubject(b, 50, false) }
func BenchmarkOwnPermissionsCached200(b *testing.B) { benchSameSubject(b, 200, false) }
