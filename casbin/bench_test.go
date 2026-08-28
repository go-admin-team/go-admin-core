package mycasbin

import (
	"strconv"
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

	m, err := model.NewModelFromString(text)
	if err != nil {
		b.Fatal(err)
	}
	e, err := casbin.NewSyncedEnforcer(m)
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

func benchEnforce(b *testing.B, n int, sub, obj, act string, want bool) {
	e := benchEnforcer(b, n)

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
