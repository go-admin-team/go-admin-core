package mycasbin

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// A policy written by another instance reaches this one only through a reload.
// Setup used to load once and never again, so a permission granted on one
// process stayed invisible to every other until it restarted.
//
// The write here goes straight to the table, which is what the other instance's
// adapter would have done.
func TestSetupReloadsPolicyWrittenElsewhere(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	ReloadInterval = 50 * time.Millisecond
	e := Setup(db, "")

	const sub, obj, act = "alice", "/data", "GET"

	allowed, err := e.Enforce(sub, obj, act)
	if err != nil {
		t.Fatalf("Enforce: %v", err)
	}
	if allowed {
		t.Fatal("the policy is empty and the request was allowed")
	}

	if err := db.Exec(
		`INSERT INTO casbin_rule (ptype, v0, v1, v2) VALUES (?, ?, ?, ?)`,
		"p", sub, obj, act,
	).Error; err != nil {
		t.Fatalf("insert: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		allowed, err = e.Enforce(sub, obj, act)
		if err != nil {
			t.Fatalf("Enforce: %v", err)
		}
		if allowed {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Error("a policy written outside this process never arrived: the enforcer is not reloading")
}
