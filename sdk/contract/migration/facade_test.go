package migration_test

import (
	"testing"

	"gorm.io/gorm"

	"github.com/go-admin-team/go-admin-core/v2/sdk/contract/migration"
)

// This init() plays the part of a third-party application's migration file:
// it imports nothing but this package, exactly as a real app that only
// requires go-admin-core would. Before the package-level ForApp/SetVersion/
// Snapshot existed, an application in this position had no way to reach the
// host's registry at all - NewRegistry() only ever hands back a fresh,
// private instance, and the host's own is not importable from outside
// go-admin/cmd/migrate/migration. That was the actual F9 gap dev-docs found:
// F9 had been verified from the host's side (can the execution engine
// rebuild itself from Snapshot?) and never from the application's side (can
// an application that only imports core reach a registry the host will
// read at all?).
func init() {
	migration.ForApp("probeapp").SetVersion("1786900000000", func(db *gorm.DB, version, appCode string) error {
		return nil
	})
}

// TestPackageLevelForAppReachesTheHostsRegistry is F9's actual scenario: an
// application's init(), reached through nothing but the package-level
// facade, ends up visible in the same Snapshot the host's execution engine
// reads. See the counterproof recorded in PRD 006's dev report: removing
// ForApp/SetVersion/Snapshot at package scope makes the init() above fail
// to compile, which is exactly today's state without this file's
// production counterpart.
func TestPackageLevelForAppReachesTheHostsRegistry(t *testing.T) {
	entries := migration.Snapshot()
	entry, ok := entries["probeapp-1786900000000"]
	if !ok {
		t.Fatalf("the default registry does not contain what init() registered through the package-level ForApp; got %v", entries)
	}
	if entry.AppCode != "probeapp" {
		t.Errorf("Entry.AppCode = %q, want probeapp", entry.AppCode)
	}
}

// TestPackageLevelSetVersionRegistersUnderTheFrameworkCode covers the other
// package-level entry point: the framework's own migrations
// (go-admin/cmd/migrate/migration/version/*.go) call SetVersion directly
// rather than going through ForApp.
func TestPackageLevelSetVersionRegistersUnderTheFrameworkCode(t *testing.T) {
	migration.SetVersion("1786700009500", func(db *gorm.DB, version string) error { return nil })

	entries := migration.Snapshot()
	entry, ok := entries["1786700009500"]
	if !ok {
		t.Fatalf("no entry for 1786700009500; got %v", entries)
	}
	if entry.AppCode != "" {
		t.Errorf("Entry.AppCode = %q, want empty (framework)", entry.AppCode)
	}
}
