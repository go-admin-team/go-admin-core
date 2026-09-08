// Package app lets an application declare, once, what it is: a stable
// identity (Code), a human-readable name and version, and the other
// applications it depends on. That declaration is what a host's installer
// reads to answer "what applications exist, and at what version" - a gap
// PRD 008 (go-admin) calls G1: today an application's existence is only
// inferred from the migrations it happens to have registered, with no
// name, version, author, or dependency list recorded anywhere.
//
// Code is shared with sdk/contract/migration.ForApp and
// sdk/contract/seed.SeedMenus: one application, one code, normalized the
// same way (migration.NormalizeAppCode) so a string typed on a CLI flag
// matches what all three registries were called with.
//
// core does not read Requires, Pricing, or License in this batch - it only
// carries them. Requires exists so a host's installer can refuse to install
// an application ahead of its declared dependencies rather than guess at an
// order; Pricing and License are reserved for a later authorization stage
// (PRD 003, PRD 008 open question 1) so that a first batch of published
// applications does not force a breaking change once that stage arrives.
package app

import (
	"sync"

	"github.com/go-admin-team/go-admin-core/v2/sdk/contract/migration"
)

// Manifest is what an application declares about itself once, at process
// init(), independently of how many migration files or seed.SeedMenus
// calls it makes.
type Manifest struct {
	// Code is the same identifier passed to migration.ForApp(code) and
	// seed.SeedMenus(tx, appCode, ...). Register normalizes it the same way
	// migration.ForApp does, so all three registries agree on one spelling
	// regardless of the case or whitespace the caller wrote it with.
	Code string
	// Name is a short, human-readable display name.
	Name string
	// Version is a semantic version string; see Compare. It is opaque to
	// this package outside of that one comparison - core does not
	// otherwise interpret it.
	Version string
	// Description and Author are free-form display text; core does not
	// interpret either one.
	Description string
	Author      string
	// Requires lists other applications' Code values this application
	// depends on. core neither validates nor orders installation by this
	// field in this batch; it only carries it for a host's installer to
	// read.
	//
	// Unlike every other field on Manifest, this one is a reference type:
	// both Register and Snapshot copy it (see cloneRequires) rather than
	// storing or returning the caller's slice header, so that mutating a
	// slice you passed to Register, or one Snapshot handed back, can never
	// reach the registry. A nil Requires stays nil through both copies,
	// rather than becoming an empty non-nil slice.
	Requires []string

	// Pricing and License are reserved for a later authorization stage
	// (PRD 003, PRD 008 open question 1). This package defines the fields
	// so a first batch of published applications does not force a
	// breaking change later, and neither reads nor interprets either one.
	Pricing string
	License string
}

// mu guards registry. Registration relies on Go's package-init ordering to
// be free of concurrent writers, the same convention migration.ForApp and
// config.RegisterExtend document - the mutex only protects Register racing
// Snapshot at run time, which should not happen but must not corrupt the
// map if it does.
var (
	mu       sync.Mutex
	registry = map[string]Manifest{}
)

// Register claims one application's identity. Call it from init(), before
// a host's installer runs - the same convention migration.ForApp and
// config.RegisterExtend use.
//
// Code is normalized through migration.NormalizeAppCode, and the empty
// string or migration.FrameworkAppCode ("core", reserved for the framework
// itself) are rejected, exactly as migration.ForApp rejects them - this
// package must not invent a second normalization or reservation rule that
// quietly diverges from the one three other packages already share.
//
// Name and Version are required. Registering the same code twice is a
// programming error - two applications, or two init() calls for the same
// application, silently taking turns owning one identity, is worse than a
// panic naming the offender - and panics immediately, the same convention
// config.RegisterExtend uses for a duplicate key.
func Register(m Manifest) {
	code := migration.NormalizeAppCode(m.Code)
	switch code {
	case "":
		panic("app: Register called with an empty Code")
	case migration.FrameworkAppCode:
		panic("app: Register: code " + migration.FrameworkAppCode + " is reserved for the framework")
	}
	if m.Name == "" {
		panic("app: Register: Name is required")
	}
	if m.Version == "" {
		panic("app: Register: Version is required")
	}

	mu.Lock()
	defer mu.Unlock()
	if _, dup := registry[code]; dup {
		panic("app: Register called twice for code " + code)
	}
	m.Code = code
	// Requires is the one field on Manifest that is a reference type: if it
	// were stored as-is, the caller would still be holding the same slice
	// header, and mutating it after Register returns would silently rewrite
	// an already-registered application's dependency list. Clone it into a
	// slice this package alone holds a reference to, the same reasoning
	// Snapshot's copy of Requires below documents for the read side.
	m.Requires = cloneRequires(m.Requires)
	registry[code] = m
}

// Snapshot returns a copy of every registered manifest, keyed by Code - the
// only way a host reads this registry, the same reasoning
// migration.Registry.Snapshot documents: a copy, not the live map, so a
// host can iterate it without holding this package's lock across a
// database call, mutating what it received cannot corrupt the registry,
// and a Register call made after a Snapshot was taken cannot retroactively
// appear in it.
//
// Copying the map is not enough on its own: a map holding Manifest values
// by value still leaves every value's Requires slice pointing at the same
// backing array the registry itself uses, because copying a struct copies
// a slice's header, not what the header points to. Without also cloning
// Requires per entry, a caller mutating one element of a returned
// snapshot's Requires would reach through to the registry's own copy -
// and, from there, into every other snapshot ever taken, since they would
// all still be aliasing that one array. Each returned Manifest gets its
// own Requires clone below for the same reason Register clones the
// caller's slice on the write side.
func Snapshot() map[string]Manifest {
	mu.Lock()
	defer mu.Unlock()
	out := make(map[string]Manifest, len(registry))
	for k, v := range registry {
		v.Requires = cloneRequires(v.Requires)
		out[k] = v
	}
	return out
}

// cloneRequires returns an independent copy of req: a new slice with its
// own backing array, so neither Register nor Snapshot ever hands out, or
// stores, a slice header that still aliases a caller's own slice or the
// registry's. A nil req returns nil rather than an empty non-nil slice, so
// a Manifest that never set Requires round-trips through Register and
// Snapshot exactly as the zero value - nil, not []string{} - rather than
// this package inventing a distinction the caller never made.
func cloneRequires(req []string) []string {
	if req == nil {
		return nil
	}
	out := make([]string, len(req))
	copy(out, req)
	return out
}
