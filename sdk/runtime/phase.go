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
