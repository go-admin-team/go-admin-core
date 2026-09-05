package runtime

import "strconv"

// Phase names a point in the life of the application that a module can attach
// work to.
//
// The values are assigned explicitly rather than with iota, and with gaps: a
// phase inserted later takes a number in a gap instead of moving the numbers
// already compiled into somebody else's binary, and comparing two phases with
// < keeps meaning "happens earlier". The numbers are an implementation
// detail. They are not part of the contract, and must not be persisted, sent
// over the wire, or written into configuration - name the constant instead.
type Phase int

const (
	// AfterResource is reached once the database, cache, queue and casbin
	// enforcer have been built from the configuration that is current at that
	// moment.
	//
	// It is the one phase that runs more than once: a configuration reload
	// rebuilds those resources, and the point of the phase is to give the
	// code that depends on them a chance to attach itself to the new ones. A
	// hook registered here must therefore be written to run repeatedly - see
	// RunPhase for what "repeatedly" is allowed to mean.
	AfterResource Phase = 10

	// BeforeRouter is reached before the router is initialised, which is the
	// last point at which a module can still affect how routes are built.
	//
	// It is not the same point as the SetBefore registry: those callbacks run
	// after the router has been initialised, not before it.
	BeforeRouter Phase = 20

	// AfterListen is reached once the listening socket is accepting
	// connections - not merely once the goroutine that serves it was started.
	// A hook here can rely on the port being bound, because a failure to bind
	// has already been reported by then.
	AfterListen Phase = 30

	// BeforeExit is reached after the HTTP server has been shut down and
	// before the process returns, for work that must happen on the way out
	// but does not need the server.
	//
	// It is not a place to drain a load balancer: by the time it runs the
	// listener is already closed, so anything that depends on still being
	// reachable belongs somewhere this package does not currently offer.
	BeforeExit Phase = 40
)

// String names the phase, so that a log line reads AfterResource rather than
// 10. An unknown value prints as Phase(42) rather than as a wrong name, which
// is what the out-of-range reports in SetPhase and RunPhase carry.
func (p Phase) String() string {
	switch p {
	case AfterResource:
		return "AfterResource"
	case BeforeRouter:
		return "BeforeRouter"
	case AfterListen:
		return "AfterListen"
	case BeforeExit:
		return "BeforeExit"
	}
	return "Phase(" + strconv.Itoa(int(p)) + ")"
}

// phaseCount is how many phases there are, and therefore how many registries
// an Application carries.
//
// They live in a fixed-size array rather than a map because every field of
// Application is unexported, which makes &runtime.Application{} legal outside
// this package, and NewConfig does not initialise the before and appRouters
// registries either - the zero value is expected to work. A nil map would
// crash on the first SetPhase instead.
const phaseCount = 4

// index maps a phase to its registry slot, reporting false for a value that
// is not one of the four constants.
//
// It is a switch and not arithmetic on int(p)/10-1: that would hand Phase(11)
// the AfterResource slot, so a caller who got a constant slightly wrong would
// silently attach to a phase they did not name.
func (p Phase) index() (int, bool) {
	switch p {
	case AfterResource:
		return 0, true
	case BeforeRouter:
		return 1, true
	case AfterListen:
		return 2, true
	case BeforeExit:
		return 3, true
	}
	return 0, false
}

// SetPhase registers a callback to run when the application reaches p.
//
// The callback runs when RunPhase(p) is called, in registration order, behind
// the same panic guard as SetBefore: a panic is logged with the registration
// site and the remaining callbacks still run, unless WithFatal says otherwise.
//
// Registering after that phase has already run is refused and reported, and
// so is a p that is not one of the four constants - neither panics, because
// an installer registering a hook is usually running inside an init() where a
// panic takes the whole program down and names this file rather than the
// caller. Ask PhaseSealed if you need to know rather than read it in a log.
func (e *Application) SetPhase(p Phase, f func(), opts ...CallbackOption) {
	site := callerSite(2)
	i, ok := p.index()
	if !ok {
		e.log().Errorf("runtime: SetPhase ignored - %v is not a life-cycle phase; "+
			"use AfterResource, BeforeRouter, AfterListen or BeforeExit (registered at %s)", p, site)
		return
	}
	e.register(&e.phases[i], newCallback(f, site, opts),
		"SetPhase("+p.String()+")", "RunPhase("+p.String()+")")
}

// RunPhase executes every callback registered for p that has not run yet, in
// registration order, and closes the phase to further registration.
//
// Calling it again is a no-op: the callbacks that already ran do not run
// twice. The callbacks run with no lock held, because they routinely call
// back into Application and sync.RWMutex is not reentrant.
//
// A p that is not one of the four constants is reported and does nothing.
func (e *Application) RunPhase(p Phase) {
	i, ok := p.index()
	if !ok {
		e.log().Errorf("runtime: RunPhase(%v) did nothing - that is not a life-cycle phase; "+
			"use AfterResource, BeforeRouter, AfterListen or BeforeExit", p)
		return
	}
	// The getter argument is empty because no method hands phase callbacks
	// out for the caller to loop over, so the warning it guards is
	// unreachable here. That omission is deliberate: GetBefore and
	// GetAppRouters showed what handing them out costs.
	e.runRegistry(&e.phases[i], p.String(), "", "RunPhase("+p.String()+")")
}

// PhaseSealed reports whether p has already run, i.e. whether a further
// SetPhase for it would be dropped.
//
// A phase that is not one of the four constants reports true: "your callback
// will not run in that phase" is the answer that keeps a caller from
// registering into a hole, and it is also reported, because asking about a
// phase that does not exist is a mistake in the caller either way.
func (e *Application) PhaseSealed(p Phase) bool {
	i, ok := p.index()
	if !ok {
		e.log().Errorf("runtime: PhaseSealed(%v) reports sealed - that is not a life-cycle phase; "+
			"use AfterResource, BeforeRouter, AfterListen or BeforeExit", p)
		return true
	}
	e.mux.RLock()
	defer e.mux.RUnlock()
	return e.phases[i].sealed
}
