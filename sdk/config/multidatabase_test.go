package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// captureWarnings redirects the default logger for one test.
func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()

	buf := &bytes.Buffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return buf
}

// A single-database configuration is expressed as the wildcard entry, which is
// how one database comes to serve every tenant.
func TestSingleDatabaseBecomesTheWildcard(t *testing.T) {
	dbs := map[string]*Database{}
	c := &Config{Database: &Database{Driver: "mysql"}, Databases: &dbs}

	c.multiDatabase()

	if len(dbs) != 1 || dbs["*"] == nil {
		t.Errorf("got %#v, want a single wildcard entry", dbs)
	}
}

// The wildcard short-circuits the per-tenant lookup, so entries listed beside it
// never serve anyone. That is a configuration mistake with no symptom other than
// tenants sharing a database, which is exactly the kind that has to be said out
// loud.
func TestWildcardBesidePerTenantEntriesWarns(t *testing.T) {
	buf := captureWarnings(t)

	dbs := map[string]*Database{
		"*":        {Driver: "mysql"},
		"tenant-a": {Driver: "mysql"},
		"tenant-b": {Driver: "mysql"},
	}
	c := &Config{Database: &Database{}, Databases: &dbs}

	c.multiDatabase()

	got := buf.String()
	if !strings.Contains(got, "wildcard") {
		t.Errorf("the clash must be reported, got: %q", got)
	}
	for _, name := range []string{"tenant-a", "tenant-b"} {
		if !strings.Contains(got, name) {
			t.Errorf("the warning must name %q so it can be found, got: %q", name, got)
		}
	}
}

// Per-tenant entries on their own are the normal multi-tenant setup and must
// stay quiet, or the warning becomes noise nobody reads.
func TestPerTenantEntriesAloneAreSilent(t *testing.T) {
	buf := captureWarnings(t)

	dbs := map[string]*Database{"tenant-a": {Driver: "mysql"}, "tenant-b": {Driver: "mysql"}}
	c := &Config{Database: &Database{}, Databases: &dbs}

	c.multiDatabase()

	if got := buf.String(); got != "" {
		t.Errorf("a normal multi-tenant configuration must not warn, got: %q", got)
	}
}
