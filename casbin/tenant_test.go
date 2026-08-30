package mycasbin

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// openTenantDB opens a named in-memory database. The name matters: the
// anonymous "file::memory:" form is shared process-wide, which would make two
// tenants the same database and hide exactly what this test is checking.
func openTenantDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	return db
}

func grant(t *testing.T, db *gorm.DB, sub, obj, act string) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO casbin_rule (ptype, v0, v1, v2) VALUES (?, ?, ?, ?)`,
		"p", sub, obj, act,
	).Error; err != nil {
		t.Fatalf("insert: %v", err)
	}
}

// A deployment can serve several databases - one per host - and Setup used to
// build the enforcer under a sync.Once. The first tenant to arrive built it,
// every later tenant got that same enforcer back, and none of them ever loaded
// their own casbin_rule table. Requests for tenant B were then decided by
// tenant A's policy: an authorization granted in one tenant applied in the
// other, and one denied in A stayed denied in B however B was configured.
func TestSetupGivesEachTenantItsOwnPolicy(t *testing.T) {
	ReloadInterval = 0 // reload explicitly, so the assertions are not timing-dependent

	dbA := openTenantDB(t, "casbin-tenant-a")
	dbB := openTenantDB(t, "casbin-tenant-b")

	eA := Setup(dbA, "tenant-a.example")
	eB := Setup(dbB, "tenant-b.example")

	if eA == eB {
		t.Fatal("both tenants share one enforcer, so one tenant's policy decides the other's requests")
	}

	const sub, obj, act = "alice", "/data", "GET"
	grant(t, dbA, sub, obj, act)
	if err := eA.LoadPolicy(); err != nil {
		t.Fatalf("tenant A LoadPolicy: %v", err)
	}
	if err := eB.LoadPolicy(); err != nil {
		t.Fatalf("tenant B LoadPolicy: %v", err)
	}

	allowed, err := eA.Enforce(sub, obj, act)
	if err != nil {
		t.Fatalf("tenant A Enforce: %v", err)
	}
	if !allowed {
		t.Error("tenant A granted the permission and was denied")
	}

	allowed, err = eB.Enforce(sub, obj, act)
	if err != nil {
		t.Fatalf("tenant B Enforce: %v", err)
	}
	if allowed {
		t.Error("tenant B allowed a request its own casbin_rule table never granted")
	}
}

// The cache still has to be a cache: repeated calls for one tenant must not
// build a second enforcer, which would leave two policy reload loops running
// against the same database.
func TestSetupReusesTheEnforcerForOneTenant(t *testing.T) {
	ReloadInterval = 0

	db := openTenantDB(t, "casbin-tenant-repeat")
	first := Setup(db, "repeat.example")
	if second := Setup(db, "repeat.example"); second != first {
		t.Error("Setup built a second enforcer for a tenant it had already built one for")
	}
}
