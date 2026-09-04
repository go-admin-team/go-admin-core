// Package seed lets an application register the menu and API entries it
// needs in order to appear in the admin UI, without knowing what table those
// entries end up in.
//
// core deliberately does not define sys_menu, sys_api, or any of the tables
// a host wires them to (sys_menu_api_rule, sys_role_menu, casbin_rule). Two
// versions of a menu row already exist in go-admin today, disagreeing on
// soft-delete shape, and the difference has caused real bugs; a third copy
// living in core - the one package with no repository-local tool watching it
// - would be strictly worse; see docs/contract.md for the full argument.
// Instead, an application describes what it wants (a MenuSpec, an ApiSpec)
// and the host, through the Seeder it registers, decides how that becomes
// rows in its own schema. MenuSpec.Kind reuses the three menu_type values
// sdk/contract/models already defines (models.Directory, models.Menu,
// models.Button) rather than a second, identically-valued type local to this
// package - core makes one promise about what those three strings are, not
// two.
//
// This boundary is about API evolution, not about safety - the application
// still runs inside the host process holding the same *gorm.DB the host
// does, and could write to any table directly regardless of what this
// package offers. See the security note on Seeder.
package seed

import (
	"errors"
	"fmt"
	"sync"

	"gorm.io/gorm"
)

// MenuSpec is what an application asks the host to seed as one node of its
// menu tree. Its fields are what rendering a menu and checking a
// button-level permission need, not a mirror of any table's columns - the
// host is free to add columns to its own sys_menu without this type
// changing shape.
type MenuSpec struct {
	// Code identifies this node among the specs passed in the same call. It
	// is not a primary key and is never written verbatim to a database
	// column: the host decides how ids are assigned. It exists purely so
	// Parent and ApiCodes can refer to other specs in this batch without the
	// application needing to invent or guess a numeric id.
	Code string
	// Parent is another MenuSpec's Code in the same call, or "" for a
	// top-level node.
	Parent string
	// Kind is one of models.Directory, models.Menu or models.Button - the
	// same three values sys_menu.menu_type already uses, reused here rather
	// than redefined so that a fork does not have to choose which of two
	// identically-valued constants to write.
	Kind  string
	Title string
	// Path is the route path. Meaningless for models.Button.
	Path string
	// Component is the frontend component path. Meaningless for
	// models.Directory (conventionally a bare layout) and models.Button.
	//
	// For a packaged application this must start with "apps/<code>/" - the
	// frontend tells a packaged view from a view built into the host by that
	// prefix alone, and anything else is read as the latter. A path that
	// looks like a host view but is not one does not error: the frontend
	// goes looking for it, does not find it, and falls back to its
	// not-installed placeholder, which reports nothing about a bad
	// Component value. core does not enforce the prefix - Component is an
	// opaque string here - so this convention only holds if callers follow
	// it; see docs/contract.md's example.
	Component string
	// Icon is the sidebar icon name. Meaningless for models.Button.
	Icon string
	// Permission is the button-level identifier checked against the
	// frontend's v-permisaction directive. Meaningless for models.Directory
	// and models.Menu.
	Permission string
	// Sort orders siblings under the same Parent.
	Sort int
	// ApiCodes lists ApiSpec.Code values, from the apis passed in the same
	// call, that this node should be linked to (sys_menu_api_rule in
	// go-admin), so that granting a role this menu also grants the APIs the
	// page needs. Leave empty for models.Directory and models.Button nodes.
	ApiCodes []string
}

// ApiSpec is what an application asks the host to register as one API route
// for Casbin to authorize against (sys_api, and indirectly casbin_rule).
type ApiSpec struct {
	// Code identifies this spec among the apis passed in the same call; see
	// MenuSpec.ApiCodes. It is not written to any column.
	Code   string
	Title  string
	Path   string
	Method string
	// Handle is a human-readable identifier for the handler this route
	// calls, conventionally "<package>.<Type>.<Method>-fm". It is
	// informational: nothing in this package or the host is required to
	// parse it.
	Handle string
}

// Seeder is implemented by the host. It receives everything one application
// asked to seed, in one call, and decides how those specs become rows across
// whichever tables its own schema uses.
//
// Security note, not a suggestion: this boundary does not sandbox anything.
// The tx passed to SeedMenus is a full *gorm.DB - the same handle an
// application's migration already holds outside this call - so an
// application that wanted to write sys_menu, sys_api, sys_menu_api_rule, or
// casbin_rule directly, bypassing Seeder entirely, always could. There is
// also an indirect route that does not touch casbin_rule at all: linking a
// menu to another application's (or the host's own) API through ApiCodes,
// then waiting for an administrator to grant that menu to a role through the
// ordinary admin UI, which generates the matching Casbin policy on its own.
// Installing an application means trusting it with the host's database
// connection, at the same level of trust as importing any other Go package
// into the binary. Seeder exists so a well-behaved application does not need
// to know go-admin's schema to be well-behaved, not so a malicious one is
// contained.
type Seeder interface {
	SeedMenus(tx *gorm.DB, appCode string, menus []MenuSpec, apis []ApiSpec) error
}

var (
	seederMu sync.RWMutex
	seeder   Seeder
)

// RegisterSeeder installs the host's Seeder implementation. There is exactly
// one seeder in a process - unlike RegisterExtend's per-key registry, a
// Seeder is not scoped to an application, since it owns tables no individual
// application owns - so a second call is treated as a programming error
// rather than a silent replacement: whichever call happened to run last
// would otherwise win with nothing to say that the first implementation was
// ever discarded.
//
// Call it once, from the host's own init() or startup code, before any
// application's migration calls SeedMenus.
func RegisterSeeder(s Seeder) {
	if s == nil {
		panic("seed: RegisterSeeder called with a nil Seeder")
	}
	seederMu.Lock()
	defer seederMu.Unlock()
	if seeder != nil {
		panic("seed: RegisterSeeder called twice; exactly one Seeder may be registered per process")
	}
	seeder = s
}

// ErrNoSeeder is returned by SeedMenus when no host has called
// RegisterSeeder.
var ErrNoSeeder = errors.New("seed: no Seeder registered; the host must call seed.RegisterSeeder before any application migration calls SeedMenus")

// SeedMenus asks the registered host Seeder to turn menus and apis into
// rows, inside the caller's own transaction. Every application-authored
// migration that needs a menu, an API, or both is expected to go through
// this function rather than writing to sys_menu / sys_api directly - see the
// security note on Seeder for what that boundary does and does not protect
// against.
//
// appCode identifies which application these specs belong to; it is
// required so that the host can attribute, and later find or remove, rows
// one specific application created. It is part of the signature rather than
// a later addition because adding a parameter here would be a breaking
// change to every existing caller.
//
// It returns ErrNoSeeder rather than panicking when nothing has registered:
// an application's migration should get an ordinary error it can report,
// not a crash, from running against a host that has not wired up seeding.
func SeedMenus(tx *gorm.DB, appCode string, menus []MenuSpec, apis []ApiSpec) error {
	if appCode == "" {
		return errors.New("seed: SeedMenus called with an empty appCode")
	}

	seederMu.RLock()
	s := seeder
	seederMu.RUnlock()
	if s == nil {
		return ErrNoSeeder
	}
	if err := s.SeedMenus(tx, appCode, menus, apis); err != nil {
		return fmt.Errorf("seed: app %q: %w", appCode, err)
	}
	return nil
}
