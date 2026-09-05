package runtime

import (
	"os"
	goruntime "runtime" // aliased: this package is itself called runtime
	"runtime/debug"
	"slices"
	"strconv"

	"github.com/go-admin-team/go-admin-core/v2/logger"
)

// callback is one registered startup function plus everything the guard needs
// in order to say something useful when it panics.
//
// site is recorded at registration rather than at execution because a panic
// unwinds to this file: without it the log names the framework, and the reader
// still has to find out who registered the thing that failed.
type callback struct {
	fn    func()
	name  string // caller-supplied label; empty unless WithName was used
	site  string // "file:line" of the SetXxx call that registered it
	fatal bool
}

// label is "name (file:line)", or "(file:line)" when the callback is unnamed.
func (cb callback) label() string {
	if cb.name == "" {
		return "(" + cb.site + ")"
	}
	return cb.name + " (" + cb.site + ")"
}

// registry holds one kind of startup callback.
//
// cursor is what makes Run* idempotent: entries before it have already been
// dispatched, so a second Run finds nothing to do. sealed is the other half:
// once the registry has run, a late registration is dropped and reported,
// instead of sitting in a slice that nobody will ever walk again.
//
// Every method here assumes the caller already holds Application.mux.
// sync.RWMutex is not reentrant, so none of them may take it.
//
// A re-entrant phase uses none of cursor and sealed - it runs every entry
// again on every round - and uses running, pending, rounds and lastCount
// instead. Those four are described where the rounds are actually driven, in
// runReentrant.
type registry struct {
	entries []callback
	cursor  int
	sealed  bool
	handed  bool // Get* handed the entries out for a loop core cannot see

	running   bool // a round is in flight
	pending   bool // a trigger arrived during that round and owes another one
	rounds    int  // rounds completed, so the first one has nothing to compare against
	lastCount int  // len(entries) at the start of the previous round
}

// appendLocked records a callback, reporting false when the registry is closed.
func (r *registry) appendLocked(cb callback) bool {
	if r.sealed {
		return false
	}
	r.entries = append(r.entries, cb)
	return true
}

// snapshotLocked returns the registered functions in registration order.
//
// It is a copy. Handing back the live slice let the caller range over it after
// the lock was gone while another goroutine appended to it - the same defect
// GetAllDb was fixed for.
func (r *registry) snapshotLocked() []func() {
	if len(r.entries) == 0 {
		// nil rather than an empty slice: that is what the field itself used
		// to return, and a caller comparing the result against nil should not
		// start seeing a different answer.
		return nil
	}
	out := make([]func(), len(r.entries))
	for i := range r.entries {
		out[i] = r.entries[i].fn
	}
	return out
}

// takePendingLocked claims everything not yet dispatched and closes the
// registry, both in the caller's single critical section.
//
// Doing all three together is what removes the middle state: a concurrent
// SetXxx either lands in this batch or is refused as late, never neither.
func (r *registry) takePendingLocked() []callback {
	pending := slices.Clone(r.entries[r.cursor:])
	r.cursor = len(r.entries)
	r.sealed = true
	return pending
}

// takeAllLocked returns every registered callback, without sealing the
// registry and without moving the cursor.
//
// That is the difference between a phase that happens once and one that
// happens again after a configuration reload: the hooks worth running the
// second time are precisely the ones that already ran the first time. They
// are the code that attaches itself to the database, the cache and the queue,
// and those have just been rebuilt.
func (r *registry) takeAllLocked() []callback {
	return slices.Clone(r.entries)
}

// runReentrant runs every registered callback, and keeps running rounds for
// as long as triggers keep arriving, never two rounds at once.
//
//	in flight? -> record that another round is owed, and return
//	otherwise  -> take the entries under the lock, run them without it,
//	              then take the lock again and go round once more if one
//	              is owed
//
// Dropping the second trigger instead would be the cheaper thing to write and
// the wrong thing to ship: the resource that the second reload built may have
// been installed after the first round took its snapshot, so dropping it
// leaves that resource with none of the consumers that were supposed to
// attach to it - a queue nobody reads, which is the defect this whole
// mechanism exists to close.
//
// A caller whose trigger only set the flag returns before the work is done.
// Waiting instead would block the single goroutine that drives configuration
// reloads behind hooks of unknown duration, so a re-entrant phase is the one
// phase whose RunPhase is not always synchronous.
//
// Callbacks registered while a round is running are not picked up by that
// round; the next one takes them. Sweeping them up at the end instead would
// never terminate for a hook that registers a hook.
func (e *Application) runReentrant(r *registry, kind string) {
	e.mux.Lock()
	if r.running {
		r.pending = true
		e.mux.Unlock()
		return
	}
	r.running = true
	e.mux.Unlock()

	for {
		e.mux.Lock()
		batch := r.takeAllLocked()
		grew := r.rounds > 0 && len(batch) > r.lastCount
		previous := r.lastCount
		r.lastCount = len(batch)
		r.rounds++
		e.mux.Unlock()

		log := e.log()
		if grew {
			// The registry of a re-entrant phase only ever grows, and every
			// entry runs on every round. A hook that registers a hook
			// therefore costs one more callback per reload for the life of
			// the process, and nothing else in this package can notice it:
			// there is no failure, only a number going up.
			log.Warnf("runtime: %s has %d callbacks, up from %d since the last round - "+
				"a hook that registers a hook grows this registry on every configuration reload; "+
				"if this number keeps climbing, find the hook that is not idempotent", kind, len(batch), previous)
		}
		for i := range batch {
			e.invoke(kind, batch[i], log)
		}

		e.mux.Lock()
		if !r.pending {
			r.running = false
			e.mux.Unlock()
			return
		}
		r.pending = false
		e.mux.Unlock()
	}
}

// CallbackOption configures a callback registered through SetBeforeWith or
// SetAppRoutersWith.
//
// Passing no options is the historical behaviour: a panic is logged and the
// remaining callbacks still run. Nothing a caller does not ask for changes.
type CallbackOption func(*callbackConfig)

type callbackConfig struct {
	name  string
	fatal bool
}

// WithFatal marks a callback the process must not start without: a panic in it
// is logged and then the process exits with status 1.
//
// This is the exception, not the default. An application that marks itself
// fatal makes the whole host program as fragile as itself, and the exit skips
// every deferred cleanup - so it only belongs on work that happens before the
// server starts listening.
func WithFatal() CallbackOption { return func(c *callbackConfig) { c.fatal = true } }

// WithName labels the callback in guard logs. Without it the log carries the
// registration site only, which is enough to find the code but not enough to
// know what it was for.
func WithName(name string) CallbackOption { return func(c *callbackConfig) { c.name = name } }

func newCallback(f func(), site string, opts []CallbackOption) callback {
	cfg := callbackConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	return callback{fn: f, name: cfg.name, site: site, fatal: cfg.fatal}
}

// callerSite reports where a registration came from.
//
// skip is counted at the exported method on purpose - 0 is this function, 1 is
// SetBefore, 2 is whoever called it. Computing it any deeper means every new
// layer of indirection silently moves the answer.
func callerSite(skip int) string {
	_, file, line, ok := goruntime.Caller(skip)
	if !ok {
		return "unknown"
	}
	return file + ":" + strconv.Itoa(line)
}

// exit is os.Exit behind a seam so the fatal path can be asserted in-process
// as well as in the subprocess test.
var exit = os.Exit

// log returns a helper over the configured logger.
//
// The nil fallback matters because every caller of this is already reporting a
// defect: a program that called SetLogger(nil) should still be told that its
// callback panicked, not crash a second time on the way to saying so.
func (e *Application) log() *logger.Helper {
	l := e.GetLogger()
	if l == nil {
		l = logger.NewLogger()
	}
	return logger.NewHelper(l)
}

// register records a callback, or reports that it arrived too late.
//
// The lock covers the append and nothing else. Logging under it would run a
// logger the application supplied, and there is no promise that it will not
// call back into Application.
func (e *Application) register(r *registry, cb callback, setter, runner string) {
	e.mux.Lock()
	ok := r.appendLocked(cb)
	e.mux.Unlock()
	if ok {
		return
	}
	e.log().Errorf("runtime: %s ignored - the registry was already run and closed by %s; "+
		"register from init() or before the server starts (registered at %s)", setter, runner, cb.site)
}

// runRegistry executes everything registered and not yet dispatched.
//
// Callbacks run with no lock held. They routinely call back into Application -
// an app router asks for the engine and then sets it, a config callback stores
// a database - and sync.RWMutex is not reentrant, so holding mux across a
// callback deadlocks on the first one that does.
func (e *Application) runRegistry(r *registry, kind, getter, runner string) {
	e.mux.Lock()
	pending := r.takePendingLocked()
	handed := r.handed
	e.mux.Unlock()

	if len(pending) == 0 {
		return
	}

	log := e.log()
	if handed {
		log.Warnf("runtime: %s was called before %s, and core cannot see a loop written outside it; "+
			"if you run the returned callbacks yourself they run twice - drop your own loop", getter, runner)
	}
	for i := range pending {
		e.invoke(kind, pending[i], log)
	}
}

// invoke runs one callback behind the panic guard.
//
// A callback that already recovers for itself needs no special case: the panic
// stops there, cb.fn returns normally and the recover below sees nil. The
// guard reaches exactly as far as Go's recover does - which is to say not into
// a goroutine the callback started.
func (e *Application) invoke(kind string, cb callback, log *logger.Helper) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		stack := debug.Stack()
		if cb.fatal {
			// Not log.Fatalf: Helper.Fatalf returns without exiting when the
			// configured level filters FatalLevel out, which would turn "must
			// not start" into "started anyway, quietly".
			log.Errorf("runtime: fatal %s callback %s panicked, exiting: %v\n%s",
				kind, cb.label(), r, stack)
			exit(1)
			return
		}
		log.Errorf("runtime: %s callback %s panicked, continuing with the rest: %v\n%s",
			kind, cb.label(), r, stack)
	}()
	cb.fn()
}

// SetBefore registers a callback to run before the server starts.
//
// The callback runs when RunBefore is called, in registration order, behind a
// panic guard. Registering after RunBefore has run is refused and reported.
func (e *Application) SetBefore(f func()) {
	e.register(&e.before, callback{fn: f, site: callerSite(2)}, "SetBefore", "RunBefore")
}

// SetBeforeWith registers a before callback and configures how a panic in it
// is treated. With no options it is identical to SetBefore.
func (e *Application) SetBeforeWith(f func(), opts ...CallbackOption) {
	e.register(&e.before, newCallback(f, callerSite(2), opts), "SetBeforeWith", "RunBefore")
}

// GetBefore returns the registered before callbacks, in registration order.
//
// Deprecated: use RunBefore. Running the returned callbacks yourself gets no
// panic guard and no failure levels, and core cannot see your loop: calling
// RunBefore afterwards runs everything a second time.
func (e *Application) GetBefore() []func() {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.before.handed = true
	return e.before.snapshotLocked()
}

// RunBefore executes every before callback that has not run yet, in
// registration order, and closes the registry to further registration.
//
// Calling it again is a no-op: the callbacks that already ran do not run twice.
func (e *Application) RunBefore() {
	e.runRegistry(&e.before, "before", "GetBefore", "RunBefore")
}

// BeforeSealed reports whether RunBefore has run. A registration made after
// that point is dropped, so an installer can ask rather than find out from a
// log line.
func (e *Application) BeforeSealed() bool {
	e.mux.RLock()
	defer e.mux.RUnlock()
	return e.before.sealed
}

// SetAppRouters registers a router initialiser for an application module.
//
// The callback runs when RunAppRouters is called, in registration order,
// behind a panic guard. Registering after RunAppRouters has run is refused and
// reported.
func (e *Application) SetAppRouters(appRouters func()) {
	e.register(&e.appRouters, callback{fn: appRouters, site: callerSite(2)}, "SetAppRouters", "RunAppRouters")
}

// SetAppRoutersWith registers a router initialiser and configures how a panic
// in it is treated. With no options it is identical to SetAppRouters.
func (e *Application) SetAppRoutersWith(appRouters func(), opts ...CallbackOption) {
	e.register(&e.appRouters, newCallback(appRouters, callerSite(2), opts), "SetAppRoutersWith", "RunAppRouters")
}

// GetAppRouters returns the registered router initialisers, in registration
// order.
//
// Deprecated: use RunAppRouters. Running the returned callbacks yourself gets
// no panic guard and no failure levels, and core cannot see your loop: calling
// RunAppRouters afterwards runs everything a second time.
func (e *Application) GetAppRouters() []func() {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.appRouters.handed = true
	return e.appRouters.snapshotLocked()
}

// RunAppRouters executes every router initialiser that has not run yet, in
// registration order, and closes the registry to further registration.
//
// Calling it again is a no-op: the initialisers that already ran do not run
// twice.
func (e *Application) RunAppRouters() {
	e.runRegistry(&e.appRouters, "appRouters", "GetAppRouters", "RunAppRouters")
}

// AppRoutersSealed reports whether RunAppRouters has run. A registration made
// after that point is dropped, so an installer can ask rather than find out
// from a log line.
func (e *Application) AppRoutersSealed() bool {
	e.mux.RLock()
	defer e.mux.RUnlock()
	return e.appRouters.sealed
}
