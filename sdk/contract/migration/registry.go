// Package migration is the registration face of the framework's migration
// system: an app claims an app code and registers functions under it. What
// runs those functions - reading sys_migration, ordering entries, running
// each inside a transaction, and the `migrate` / `migrate status` commands -
// stays in the host, which reads this package's Registry through Snapshot
// rather than through a shared private field. See the package's PRD (006,
// F9) for why the split lands there: registration and execution share no
// state once Snapshot returns a copy, and only registration is something a
// third-party app - which cannot reach into the host's process - needs to
// call.
package migration

import (
	"path/filepath"
	"strings"
	"sync"

	"gorm.io/gorm"
)

// AppMigrationFunc is what an app registers through ForApp. It receives
// appCode explicitly because the migration - not the framework - writes its
// own completion row, normally as the last statement inside its own
// transaction. That is what makes "the schema change and the record of it
// commit together" true, and the framework cannot insert the row on the
// migration's behalf without giving that up. Handing the code to the
// function is what stops an app's migrations from silently recording
// themselves as the framework's.
type AppMigrationFunc func(db *gorm.DB, version, appCode string) error

// Entry is one registered migration as the host's execution engine sees it.
type Entry struct {
	AppCode string
	Fn      func(db *gorm.DB, version string) error
}

// FrameworkAppCode is the name the host's `migrate status` prints for
// migrations that belong to the framework rather than to an app, and the
// name --app accepts to select them. The stored app code for those is the
// empty string; this is only the spelling humans use. It is reserved - ForApp
// rejects it - so that every group heading the host prints is also a value
// its --app flag understands.
const FrameworkAppCode = "core"

// Registry is the registration face only: it never touches a database.
// Reading sys_migration, ordering, and running the transaction is the host's
// execution engine, which reads this registry through Snapshot.
type Registry struct {
	mu      sync.Mutex
	entries map[string]Entry
}

// NewRegistry returns an empty registry. The host keeps one package-level
// instance; tests construct their own to stay isolated from each other.
func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]Entry)}
}

func (r *Registry) setVersion(k, appCode string, f func(db *gorm.DB, version string) error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[k] = Entry{AppCode: appCode, Fn: f}
}

// SetVersion registers a migration owned by the framework itself, under the
// empty app code.
func (r *Registry) SetVersion(k string, f func(db *gorm.DB, version string) error) {
	r.setVersion(k, "", f)
}

// ForApp returns a registrar that records migrations under code.
//
// The code is lower-cased: sys_migration.version sorts as ASCII, so mixed
// case would order MyApp before crm for no reason a reader could guess, and
// the two spellings would group as two different apps in `migrate status`.
//
// An empty or reserved code panics rather than falling back to the
// framework. Registration happens in init(), so this fires the first time
// the binary runs anywhere, which is the point: an app whose migrations
// quietly file themselves under the framework is exactly the class of
// silent failure this work exists to remove. Framework migrations call
// Registry.SetVersion directly.
func (r *Registry) ForApp(code string) *AppRegistrar {
	code = NormalizeAppCode(code)
	switch code {
	case "":
		panic("migration.ForApp: empty app code; framework migrations use Registry.SetVersion")
	case FrameworkAppCode:
		panic("migration.ForApp: app code " + FrameworkAppCode + " is reserved for the framework")
	}
	return &AppRegistrar{r: r, appCode: code}
}

// AppRegistrar is a per-app view over a Registry.
type AppRegistrar struct {
	r       *Registry
	appCode string
}

// AppCode reports the code this registrar files migrations under, after
// normalisation.
func (a *AppRegistrar) AppCode() string { return a.appCode }

// SetVersion registers an app-owned migration under k, which is the bare
// timestamp taken from the file name exactly as framework migrations use.
//
// What reaches sys_migration.version is the namespaced form; the version
// string handed to f is that same namespaced string, so a migration that
// writes its own completion row keyed on version records the key the
// registry will look for next time.
func (a *AppRegistrar) SetVersion(k string, f AppMigrationFunc) {
	key := namespacedKey(a.appCode, k)
	a.r.setVersion(key, a.appCode, func(db *gorm.DB, version string) error {
		return f(db, version, a.appCode)
	})
}

// namespacedKey scopes k to appCode so two apps cannot collide on the
// sys_migration.version primary key by minting the same millisecond
// timestamp. Framework migrations (appCode == "") stay bare, matching every
// version string already in production.
func namespacedKey(appCode, k string) string {
	if appCode == "" {
		return k
	}
	return appCode + "-" + k
}

// Snapshot returns a copy of every registered entry, keyed exactly as
// sys_migration.version is. This is the only way the host's execution
// engine reads the registry: a copy, not the live map, so ordering,
// filtering, and iterating it can happen without holding the registry's
// lock across a database call.
func (r *Registry) Snapshot() map[string]Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]Entry, len(r.entries))
	for k, v := range r.entries {
		out[k] = v
	}
	return out
}

// NormalizeAppCode applies the same rule ForApp does, so a code typed on the
// command line matches one written in an init().
func NormalizeAppCode(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}

// GetFilename extracts a migration's version from its file name: the
// leading 13 characters, which is the millisecond-timestamp convention every
// migration file name follows.
func GetFilename(s string) string {
	s = filepath.Base(s)
	return s[:13]
}
