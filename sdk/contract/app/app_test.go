package app

import (
	"testing"

	"github.com/go-admin-team/go-admin-core/v2/sdk/contract/migration"
)

// resetRegistry clears the process-wide registration for the duration of
// one test. Production code has no legitimate reason to unregister an
// application - this exists so tests do not leak state into one another,
// the same reasoning sdk/contract/seed's resetSeeder documents.
func resetRegistry(t *testing.T) {
	t.Helper()
	mu.Lock()
	previous := registry
	registry = map[string]Manifest{}
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		registry = previous
		mu.Unlock()
	})
}

// An app code differing only in case or surrounding whitespace would
// register as two different applications from migration.ForApp's point of
// view, for no reason a reader could guess.
func TestRegisterNormalizesTheCode(t *testing.T) {
	resetRegistry(t)

	Register(Manifest{Code: "  CRM  ", Name: "CRM", Version: "1.0.0"})

	entries := Snapshot()
	entry, ok := entries["crm"]
	if !ok {
		t.Fatalf("no entry for \"crm\"; got %v", entries)
	}
	if entry.Code != "crm" {
		t.Errorf("Manifest.Code = %q, want \"crm\"", entry.Code)
	}
}

// Register must reject exactly what migration.ForApp rejects, for the same
// reason: an empty code has nothing to key the registry on, and "core" is
// reserved for the framework itself in that registry, so this package must
// not let an application quietly claim it here instead.
func TestRegisterRejectsReservedCodes(t *testing.T) {
	for _, code := range []string{"", "   ", migration.FrameworkAppCode, "CORE"} {
		t.Run("code="+code, func(t *testing.T) {
			resetRegistry(t)
			defer func() {
				if recover() == nil {
					t.Errorf("Register(code=%q) did not panic", code)
				}
			}()
			Register(Manifest{Code: code, Name: "x", Version: "1.0.0"})
		})
	}
}

func TestRegisterRequiresName(t *testing.T) {
	resetRegistry(t)
	defer func() {
		if recover() == nil {
			t.Error("Register with an empty Name did not panic")
		}
	}()
	Register(Manifest{Code: "order", Version: "1.0.0"})
}

func TestRegisterRequiresVersion(t *testing.T) {
	resetRegistry(t)
	defer func() {
		if recover() == nil {
			t.Error("Register with an empty Version did not panic")
		}
	}()
	Register(Manifest{Code: "order", Name: "Order"})
}

// Two applications - or two init() calls for the same one - silently
// taking turns owning one code would mean whichever ran last wins with no
// indication the first Manifest was ever discarded. A duplicate code must
// panic instead, the same convention config.RegisterExtend uses.
func TestRegisterPanicsOnDuplicateCode(t *testing.T) {
	resetRegistry(t)
	Register(Manifest{Code: "order", Name: "Order", Version: "1.0.0"})

	defer func() {
		if recover() == nil {
			t.Error("second Register for the same code did not panic")
		}
	}()
	// Different case, same normalized code - must still collide.
	Register(Manifest{Code: "ORDER", Name: "Order v2", Version: "2.0.0"})
}

// Every field a Manifest carries must reach Snapshot unchanged, including
// Requires, Pricing and License - core does not interpret the latter two in
// this batch, but it must not drop them either, since a host's installer
// (and, later, the authorization stage PRD 003 anticipates) reads them back
// from here.
func TestSnapshotReturnsEveryField(t *testing.T) {
	resetRegistry(t)
	Register(Manifest{
		Code:        "order",
		Name:        "Order",
		Version:     "1.0.0",
		Description: "order management",
		Author:      "go-admin-team",
		Requires:    []string{"crm", "payment"},
		Pricing:     "paid",
		License:     "commercial",
	})

	entries := Snapshot()
	entry, ok := entries["order"]
	if !ok {
		t.Fatalf("no entry for \"order\"; got %v", entries)
	}
	if entry.Name != "Order" || entry.Version != "1.0.0" ||
		entry.Description != "order management" || entry.Author != "go-admin-team" ||
		entry.Pricing != "paid" || entry.License != "commercial" {
		t.Errorf("Snapshot did not round-trip every field: %+v", entry)
	}
	if len(entry.Requires) != 2 || entry.Requires[0] != "crm" || entry.Requires[1] != "payment" {
		t.Errorf("Requires = %v, want [crm payment]", entry.Requires)
	}
}

// Snapshot must be a copy: a host iterates and filters it without holding
// this package's lock, so mutating what Snapshot returned must not corrupt
// the registry, and a Register call made after a Snapshot was taken must
// not retroactively appear in it - the same property
// migration.Registry.Snapshot guarantees.
func TestSnapshotIsACopy(t *testing.T) {
	resetRegistry(t)
	Register(Manifest{Code: "order", Name: "Order", Version: "1.0.0"})

	snap := Snapshot()
	delete(snap, "order")
	snap["injected"] = Manifest{Code: "injected", Name: "x", Version: "1.0.0"}

	again := Snapshot()
	if _, ok := again["order"]; !ok {
		t.Error("mutating a returned snapshot deleted the registry's own entry")
	}
	if _, ok := again["injected"]; ok {
		t.Error("mutating a returned snapshot leaked into the registry")
	}
}
