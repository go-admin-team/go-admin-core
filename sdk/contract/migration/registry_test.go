package migration

import (
	"testing"

	"gorm.io/gorm"
)

func TestNamespacedKeyLeavesFrameworkVersionsBare(t *testing.T) {
	if got := namespacedKey("", "1786700009000"); got != "1786700009000" {
		t.Errorf("framework version was rewritten to %q", got)
	}
	if got := namespacedKey("crm", "1786800001000"); got != "crm-1786800001000" {
		t.Errorf("namespacedKey = %q", got)
	}
}

// An app code differing only in case would group as two apps for the host's
// execution engine and sort before every lower-case one, for no reason a
// reader could guess.
func TestForAppNormalisesTheCode(t *testing.T) {
	r := NewRegistry()
	if got := r.ForApp("  CRM  ").AppCode(); got != "crm" {
		t.Errorf("AppCode = %q, want crm", got)
	}
}

func TestForAppRejectsReservedCodes(t *testing.T) {
	for _, code := range []string{"", "   ", FrameworkAppCode, "CORE"} {
		t.Run("code="+code, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("ForApp(%q) did not panic", code)
				}
			}()
			NewRegistry().ForApp(code)
		})
	}
}

// The registered function is handed appCode explicitly - the whole reason
// AppMigrationFunc takes three parameters instead of two - because the
// registry itself never touches a database and cannot write the migration's
// own completion row on its behalf.
func TestAppRegistrarSetVersionPassesItsAppCode(t *testing.T) {
	r := NewRegistry()
	var gotAppCode, gotVersion string
	r.ForApp("crm").SetVersion("1786800001000", func(db *gorm.DB, version, appCode string) error {
		gotVersion, gotAppCode = version, appCode
		return nil
	})

	entries := r.Snapshot()
	entry, ok := entries["crm-1786800001000"]
	if !ok {
		t.Fatalf("no entry for crm-1786800001000; got %v", entries)
	}
	if entry.AppCode != "crm" {
		t.Errorf("Entry.AppCode = %q, want crm", entry.AppCode)
	}
	if err := entry.Fn(nil, "crm-1786800001000"); err != nil {
		t.Fatalf("Fn returned %v", err)
	}
	if gotVersion != "crm-1786800001000" || gotAppCode != "crm" {
		t.Errorf("f(db, %q, %q); want the namespaced version and app code crm", gotVersion, gotAppCode)
	}
}

// The framework path stays untouched: same signature, empty app code, which
// is what sys_migration.app_code defaults to and what every row written
// before that column existed reads back as.
func TestRegistrySetVersionRecordsTheFrameworkAsEmpty(t *testing.T) {
	r := NewRegistry()
	r.SetVersion("1786700009000", func(db *gorm.DB, version string) error { return nil })

	entries := r.Snapshot()
	entry, ok := entries["1786700009000"]
	if !ok {
		t.Fatalf("no entry for 1786700009000; got %v", entries)
	}
	if entry.AppCode != "" {
		t.Errorf("Entry.AppCode = %q, want empty (framework)", entry.AppCode)
	}
}

// Snapshot must be a copy: the execution engine iterates and filters it
// without holding the registry's lock, so a caller mutating what Snapshot
// returned must not corrupt the registry, and a registration made after a
// Snapshot was taken must not retroactively appear in it.
func TestSnapshotIsACopy(t *testing.T) {
	r := NewRegistry()
	r.SetVersion("1786700009000", func(db *gorm.DB, version string) error { return nil })

	snap := r.Snapshot()
	delete(snap, "1786700009000")
	snap["injected"] = Entry{}

	again := r.Snapshot()
	if _, ok := again["1786700009000"]; !ok {
		t.Error("mutating a returned snapshot deleted the registry's own entry")
	}
	if _, ok := again["injected"]; ok {
		t.Error("mutating a returned snapshot leaked into the registry")
	}
}

// Two apps minting the same millisecond timestamp must not collide on the
// registry's key: that used to mean one of them was read as already applied
// and silently skipped. The namespace prefix is what makes that impossible.
func TestNamespacingKeepsTwoAppsWithTheSameTimestampApart(t *testing.T) {
	r := NewRegistry()
	const sameTimestamp = "1786800001000"
	for _, app := range []string{"crm", "oms"} {
		r.ForApp(app).SetVersion(sameTimestamp, func(db *gorm.DB, version, appCode string) error { return nil })
	}

	entries := r.Snapshot()
	for _, want := range []string{"crm-" + sameTimestamp, "oms-" + sameTimestamp} {
		if _, ok := entries[want]; !ok {
			t.Errorf("missing %s; got %v", want, entries)
		}
	}
}

func TestGetFilename(t *testing.T) {
	cases := map[string]string{
		"1786700009000_demo_product.go":      "1786700009000",
		"/abs/path/1786800001000_order.go":   "1786800001000",
		"version/1786700001000_demo_menu.go": "1786700001000",
	}
	for in, want := range cases {
		if got := GetFilename(in); got != want {
			t.Errorf("GetFilename(%q) = %q, want %q", in, got, want)
		}
	}
}
