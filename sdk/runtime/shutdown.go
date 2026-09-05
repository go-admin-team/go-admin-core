package runtime

import (
	"context"
	"slices"
	"sync/atomic"
)

// BeginShutdown says that the application is on its way out, and closes the
// phases that only make sense on the way in.
//
// It exists because the last configuration reload can outlive the decision to
// stop. Closing the config watcher does not wait for a reload already in
// flight, and that reload finishes by rebuilding the database, the cache and
// the queue and announcing them - which, in the middle of a shutdown, undoes
// the cleanup that has just run and starts consumers on a process that is
// being taken apart. AfterResource is also the one phase that never closes,
// so without this there is nothing to stop code registering into it while the
// process is going down.
//
// After this call SetPhase and RunPhase do nothing for every phase but
// BeforeExit, and say so. BeforeExit keeps working: it is the phase this call
// is on the way to. There is no way back - a process does not un-shut-down -
// and calling it twice is harmless.
//
// It does not interrupt a phase already running. It closes the door; it does
// not reach into the room.
//
// RunShutdown calls it, so a host that only calls RunShutdown is covered from
// that moment on. Calling it as the first step of the shutdown path is what
// covers the window before then, which is the wider one: it spans the whole
// of the HTTP server's own graceful shutdown.
func (e *Application) BeginShutdown() {
	e.shuttingDown.Store(true)
}

// SetShutdown registers a callback to run on the way out, once the server has
// stopped serving.
//
// It is the context-taking half of SetPhase(BeforeExit, ...): both register
// into the same phase, and one pass runs both. The context carries whatever
// budget the host allowed for shutdown, so a callback that can cut its work
// short - drain for what is left rather than for a fixed time - has something
// to look at. One that cannot is free to ignore it.
//
// The callbacks run in reverse registration order, which is the right default
// for a chain of hooks that built on each other: the last one registered is
// the one most likely to depend on the others. It says nothing about
// resources the host created for itself, which were not built by this chain
// and are not unwound by it.
//
// WithFatal is not honoured here, and saying so is the point: exiting from
// inside cleanup skips every callback after it, which is the opposite of what
// the option is for. The callback is kept and the option is reported, because
// refusing the registration would mean one mistyped option costs you the
// cleanup entirely - silently.
func (e *Application) SetShutdown(f func(context.Context), opts ...CallbackOption) {
	site := callerSite(2)
	cb := newCallback(nil, site, opts)
	cb.ctxFn = f
	if cb.fatal {
		cb.fatal = false
		e.log().Errorf("runtime: WithFatal ignored on the shutdown callback %s - exiting from inside "+
			"cleanup would skip every callback after it; the callback is registered without it", cb.label())
	}
	i, _ := BeforeExit.index()
	e.register(&e.phases[i], cb, "SetShutdown", "RunShutdown")
}

// RunShutdown runs the BeforeExit callbacks, in reverse registration order,
// within the caller's context, and closes the phase.
//
// It returns ctx.Err() if the context ends first, and nil otherwise. Calling
// it again runs nothing: cleanup that already ran does not run twice, because
// closing a connection pool twice is worse than not closing it.
//
// What the context bounds is the wait, not the work. The callbacks run on
// their own goroutine; when the budget is gone this method stops waiting and
// returns, and the callback that is running carries on until the process
// exits - possibly leaving whatever it was writing half-written. There is no
// way to do better: Go cannot cancel a function that does not check for it,
// which is why the callbacks are handed a context in the first place. The
// callback holding things up is named in the log.
//
// With nothing registered it returns nil immediately, without a goroutine and
// without consulting the context: nothing to run is not a timeout.
func (e *Application) RunShutdown(ctx context.Context) error {
	if ctx == nil {
		// A nil context is a bug in the caller, but these callbacks are the
		// cleanup: panicking here would skip all of them to report it.
		ctx = context.Background()
	}
	e.BeginShutdown()
	i, _ := BeforeExit.index()

	e.mux.Lock()
	pending := e.phases[i].takePendingLocked()
	e.mux.Unlock()

	if len(pending) == 0 {
		return nil
	}
	slices.Reverse(pending)

	// current is the 1-based index of the callback being run, so the timeout
	// report can name the one that is holding things up instead of saying
	// only that something did.
	var current atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		log := e.log()
		for j := range pending {
			current.Store(int32(j) + 1)
			e.invoke(BeforeExit.String(), pending[j].bound(ctx), log)
		}
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		select {
		case <-done:
			// Both were ready. Finishing in time wins over the deadline;
			// select would otherwise pick one at random and report a
			// timeout for cleanup that actually completed.
			return nil
		default:
		}
		blame := "the callbacks had not started"
		if n := current.Load(); n > 0 {
			blame = pending[n-1].label() + " was still running"
		}
		e.log().Errorf("runtime: the shutdown budget ran out - %s. The wait is over, the callback is not: "+
			"it keeps going until the process exits, and anything it half-wrote stays half-written", blame)
		return ctx.Err()
	}
}
