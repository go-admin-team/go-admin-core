package config

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

// extendSlot is what the process-wide registry holds per key, with the
// concrete T of the RegisterExtend[T] call that created it erased: the
// dispatcher below walks a map[string]extendSlot without needing to know
// what any of the registered types are, only that each can absorb one
// reload's raw JSON.
type extendSlot interface {
	// apply unmarshals data into a newly allocated value and atomically
	// publishes it. It never writes through a pointer a reader might already
	// be holding - see typedExtendSlot.
	apply(data json.RawMessage) error
}

// typedExtendSlot is what RegisterExtend[T] actually registers. current
// holds the section's most recent snapshot; apply builds the next one
// off to the side and swaps it in with a single atomic store, so a goroutine
// that already loaded a *T through Load keeps seeing that exact, complete
// value for as long as it holds the pointer, no matter what apply does
// concurrently on the next one. Nothing here ever mutates the struct a
// previous Load returned, which is what makes reading it lock-free: there is
// no field to torn-read.
type typedExtendSlot[T any] struct {
	current atomic.Pointer[T]
}

func (s *typedExtendSlot[T]) apply(data json.RawMessage) error {
	v := new(T)
	if err := json.Unmarshal(data, v); err != nil {
		return err
	}
	s.current.Store(v)
	return nil
}

// extendTargets holds every section RegisterExtend has claimed, keyed by the
// name the caller registered it under. It is process-wide because
// RegisterExtend, like SetAppRouters and migration.ForApp, is meant to be
// called from init(): registration relies on Go's package-init ordering to
// be free of concurrent writers, not on this mutex - the mutex only protects
// the read Setup does against a RegisterExtend call racing it at run time,
// which should not happen but must not corrupt the map if it does.
var (
	extendMu      sync.RWMutex
	extendTargets = map[string]extendSlot{}
)

// RegisterExtend claims one section of the extend: config tree, keyed by
// key, and returns a function that always returns that section's most
// recently loaded value as a *T - on every load and every reload,
// independently of every other registered key, so the host and any number
// of applications can each keep their own configuration without one
// overwriting another's.
//
// The returned function is the only thing a caller gets back; there is no
// target to unmarshal into, because there is nothing to synchronize. Every
// reload allocates a fresh *T from that reload's JSON and atomically
// replaces the one the accessor returns - it never writes through a pointer
// a caller might already be holding. A caller that reads the accessor once
// at startup and one that reads it from every request both get a complete,
// self-consistent value either way; neither needs a lock of its own, and T
// does not need to implement json.Unmarshaler to make that true. The
// accessor never returns nil, even before the first load completes: T's
// zero value is published immediately, so a caller does not need a nil
// check it would otherwise forget on the one code path that runs before
// Setup does its first Scan.
//
// Call it only from init(), before Setup runs - the same convention as
// SetAppRouters and migration.ForApp. Registering the same key twice is a
// programming error (two callers would silently take turns owning one
// section, and whichever ran its init() last would win with no indication
// that the other's configuration was ever read) and panics immediately
// rather than letting it happen quietly.
func RegisterExtend[T any](key string) func() *T {
	if key == "" {
		panic("config: RegisterExtend called with an empty key")
	}

	slot := &typedExtendSlot[T]{}
	slot.current.Store(new(T))

	extendMu.Lock()
	defer extendMu.Unlock()
	if _, dup := extendTargets[key]; dup {
		panic("config: RegisterExtend called twice for key \"" + key + "\"")
	}
	extendTargets[key] = slot

	return func() *T { return slot.current.Load() }
}

// extendDispatcher is what Setup installs into Config.Extend so that field -
// still declared as interface{} for the sake of code that never calls
// RegisterExtend - decodes the extend: tree per registered key instead of as
// one flat blob.
//
// It must be stored as a pointer: encoding/json only special-cases an
// interface{} field holding a non-nil pointer to something implementing
// json.Unmarshaler, which is the same mechanism the pre-existing
// ExtendConfig variable already relied on (see the back-compat branch
// below).
type extendDispatcher struct{}

// UnmarshalJSON receives the raw extend: section (or "null" if the key is
// absent) on every config load, including every reload triggered by the file
// watcher - it runs each time because Setup installs the same *extendDispatcher
// instance into Config.Extend once, and that instance's UnmarshalJSON is what
// re-publishes every registered slot's snapshot on each reload, in place.
func (*extendDispatcher) UnmarshalJSON(data []byte) error {
	// Back-compat: before RegisterExtend existed, a caller pointed the
	// package-level ExtendConfig at its own struct and got the entire
	// extend: section decoded into it. That keeps working unchanged, whether
	// or not anything is separately registered by key below - a host that
	// has not migrated to RegisterExtend is unaffected by anything that has.
	if ExtendConfig != nil {
		if err := json.Unmarshal(data, ExtendConfig); err != nil {
			return err
		}
	}

	extendMu.RLock()
	slots := make(map[string]extendSlot, len(extendTargets))
	for k, v := range extendTargets {
		slots[k] = v
	}
	extendMu.RUnlock()
	if len(slots) == 0 {
		return nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		// extend: is absent, null, or not an object - there is nothing to
		// dispatch by key. ExtendConfig's own Unmarshal above already
		// surfaced any error that matters for that path.
		return nil
	}
	for key, slot := range slots {
		section, ok := raw[key]
		if !ok {
			// A registered key absent from this particular load is normal,
			// not an error: leave its snapshot exactly as the previous
			// successful load (or the zero value, if there has been none
			// yet) left it.
			continue
		}
		if err := slot.apply(section); err != nil {
			return fmt.Errorf("config: extend section %q: %w", key, err)
		}
	}
	return nil
}
