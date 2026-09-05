package runtime

import (
	"strings"
	"sync"
	"testing"

	"github.com/go-admin-team/go-admin-core/v2/logger"
	"gorm.io/gorm"
)

// allPhases is every phase the contract names, for the tests that have to
// hold for all of them. A phase added later and left out of this list would
// go untested, which is the point of deriving nothing from it.
var allPhases = []Phase{AfterResource, BeforeRouter, AfterListen, BeforeExit}

// onceOnlyPhases is every phase that happens exactly once. AfterResource is
// not one of them: it runs again on every configuration reload, so the
// promises about sealing and about not running twice are not made for it. Its
// own promises are in phase_reentrant_test.go.
var onceOnlyPhases = []Phase{BeforeRouter, AfterListen, BeforeExit}

// Each phase runs its own callbacks, in the order they were registered, and
// nobody else's. Order is what lets one hook rely on an earlier one having
// run, so it is asserted rather than assumed.
func TestRunPhaseRunsItsOwnCallbacksInRegistrationOrder(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	ran := map[Phase][]string{}
	for _, p := range allPhases {
		for _, name := range []string{"first", "second", "third"} {
			app.SetPhase(p, func() { ran[p] = append(ran[p], name) })
		}
	}

	for _, p := range allPhases {
		app.RunPhase(p)
		if got := strings.Join(ran[p], ","); got != "first,second,third" {
			t.Errorf("%v ran %q, want first,second,third", p, got)
		}
	}
}

// Running one phase must not run another. The phases share an
// implementation, and a single registry behind all four would pass every
// order test above while running the shutdown work at start-up.
func TestRunPhaseDoesNotRunAnotherPhase(t *testing.T) {
	for _, run := range allPhases {
		app := NewConfig()
		captureLogs(t, app)

		var ran []Phase
		for _, p := range allPhases {
			app.SetPhase(p, func() { ran = append(ran, p) })
		}

		app.RunPhase(run)

		if len(ran) != 1 || ran[0] != run {
			t.Errorf("RunPhase(%v) ran %v, want exactly [%v]", run, ran, run)
		}
	}
}

// A second RunPhase is a no-op: the callbacks that already ran do not run
// twice. Cleanup registered in BeforeExit is the case that makes this matter
// - closing a pool twice is worse than not closing it.
func TestRunPhaseTwiceDoesNotRunAnythingTwice(t *testing.T) {
	for _, p := range onceOnlyPhases {
		app := NewConfig()
		captureLogs(t, app)

		calls := 0
		app.SetPhase(p, func() { calls++ })

		app.RunPhase(p)
		app.RunPhase(p)

		if calls != 1 {
			t.Errorf("%v callback ran %d times across two RunPhase calls, want 1", p, calls)
		}
	}
}

// Registering into a phase that has already run is refused and reported: the
// alternative is a callback sitting in a slice nobody will walk again, which
// is the silent failure this registry exists to remove.
func TestSetPhaseAfterTheRunIsRefusedAndReported(t *testing.T) {
	for _, p := range onceOnlyPhases {
		app := NewConfig()
		rec := captureLogs(t, app)

		app.RunPhase(p)
		if !app.PhaseSealed(p) {
			t.Errorf("%v is not sealed after RunPhase", p)
		}

		late := false
		app.SetPhase(p, func() { late = true })
		app.RunPhase(p)

		if late {
			t.Errorf("a callback registered after RunPhase(%v) ran anyway", p)
		}
		errors := rec.only(logger.ErrorLevel)
		if len(errors) != 1 {
			t.Fatalf("want one error line for the late registration into %v, got %v", p, errors)
		}
		for _, want := range []string{p.String(), "phase_registry_test.go"} {
			if !strings.Contains(errors[0], want) {
				t.Errorf("the report must name %q, got:\n%s", want, errors[0])
			}
		}
	}
}

// Before the run, a phase is open: an installer that asks gets told it can
// still register.
func TestPhaseIsNotSealedBeforeItRuns(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)
	for _, p := range allPhases {
		if app.PhaseSealed(p) {
			t.Errorf("%v reports sealed before RunPhase", p)
		}
	}
}

// A value that is not one of the four constants is a mistake in the caller,
// and it is reported as one - but it does not panic. Registration usually
// happens inside an init(), where a panic takes down the whole program and
// names this file rather than the module that got the constant wrong.
func TestSetPhaseWithAnUnknownPhaseIsDroppedAndReported(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	ran := false
	mustReturn(t, "SetPhase with an out-of-range phase", func() {
		app.SetPhase(Phase(42), func() { ran = true })
	})

	for _, p := range append(allPhases, Phase(42)) {
		app.RunPhase(p)
	}
	if ran {
		t.Error("a callback registered for an out-of-range phase ran anyway")
	}

	errors := rec.only(logger.ErrorLevel)
	if len(errors) == 0 {
		t.Fatal("dropping the callback must be reported; nothing was logged")
	}
	for _, want := range []string{"Phase(42)", "phase_registry_test.go"} {
		if !strings.Contains(errors[0], want) {
			t.Errorf("the report must contain %q, got:\n%s", want, errors[0])
		}
	}
}

// The same for the running side: reported, and nothing happens. In
// particular it must not run some other phase's callbacks.
func TestRunPhaseWithAnUnknownPhaseDoesNothingAndReports(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	ran := false
	for _, p := range allPhases {
		app.SetPhase(p, func() { ran = true })
	}

	mustReturn(t, "RunPhase with an out-of-range phase", func() { app.RunPhase(Phase(42)) })

	if ran {
		t.Error("RunPhase(Phase(42)) ran a real phase's callbacks")
	}
	errors := rec.only(logger.ErrorLevel)
	if len(errors) != 1 || !strings.Contains(errors[0], "Phase(42)") {
		t.Errorf("want one error line naming Phase(42), got %v", errors)
	}
}

// An unknown phase reports sealed. "Your callback will not run in that phase"
// is the answer that stops a caller registering into a hole; false would
// invite exactly that.
func TestPhaseSealedOfAnUnknownPhaseReportsSealed(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	for _, p := range []Phase{0, 1, 11, 42, -1} {
		if !app.PhaseSealed(p) {
			t.Errorf("PhaseSealed(%v) = false, want true for a phase that does not exist", p)
		}
	}
	if len(rec.only(logger.ErrorLevel)) != 5 {
		t.Errorf("each out-of-range question must be reported, got %v", rec.only(logger.ErrorLevel))
	}
}

// The zero value has to work. Every field of Application is unexported, so
// &runtime.Application{} is legal outside this package, and NewConfig does
// not initialise the before and appRouters registries either. A map here
// would crash on the first SetPhase - which is why the phases are an array.
func TestZeroValueApplicationSupportsPhases(t *testing.T) {
	app := &Application{}
	captureLogs(t, app)

	ran := false
	mustReturn(t, "SetPhase on a zero-value Application", func() {
		app.SetPhase(BeforeRouter, func() { ran = true })
	})
	mustReturn(t, "RunPhase on a zero-value Application", func() { app.RunPhase(BeforeRouter) })
	mustReturn(t, "PhaseSealed on a zero-value Application", func() { app.PhaseSealed(BeforeRouter) })

	if !ran {
		t.Error("the callback registered on a zero-value Application never ran")
	}
	if !app.PhaseSealed(BeforeRouter) {
		t.Error("a zero-value Application did not seal the phase it just ran")
	}
}

// The phases get the guard the other registries have: a panic is logged with
// the label and the registration site, and the rest still run.
func TestPanicInOnePhaseCallbackDoesNotStopTheRest(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	var ran []string
	app.SetPhase(AfterListen, func() { ran = append(ran, "first") })
	app.SetPhase(AfterListen, func() { panic("boom") }, WithName("metrics-exporter"))
	app.SetPhase(AfterListen, func() { ran = append(ran, "third") })

	mustReturn(t, "RunPhase with a panicking callback", func() { app.RunPhase(AfterListen) })

	if got := strings.Join(ran, ","); got != "first,third" {
		t.Errorf("ran %q, want first,third", got)
	}
	errors := rec.only(logger.ErrorLevel)
	if len(errors) != 1 {
		t.Fatalf("want one error line, got %v", errors)
	}
	for _, want := range []string{"AfterListen", "metrics-exporter", "phase_registry_test.go", "boom", "goroutine "} {
		if !strings.Contains(errors[0], want) {
			t.Errorf("the report must contain %q, got:\n%s", want, errors[0])
		}
	}
}

// Callbacks run with no lock held. They are expected to call back into
// Application - a hook that stores the new database is the whole point of
// AfterResource - and sync.RWMutex is not reentrant, so holding mux across
// one would deadlock on the first hook that does its job.
func TestPhaseCallbacksRunWithNoLockHeld(t *testing.T) {
	for _, p := range allPhases {
		app := NewConfig()
		captureLogs(t, app)

		reached := false
		app.SetPhase(p, func() {
			app.SetDb(&gorm.DB{})
			app.GetDb()
			app.PhaseSealed(p)
			reached = true
		})

		mustReturn(t, "RunPhase("+p.String()+") with a callback that calls back in", func() { app.RunPhase(p) })

		if !reached {
			t.Errorf("the %v callback did not get through calling back into Application", p)
		}
	}
}

// The phase registries are hammered from several goroutines at once, for
// -race. Concurrent registration and running is not exotic: modules register
// from init() while the process is already wiring itself up.
func TestPhaseAccessorsAreRaceFree(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	const goroutines, iters = 16, 50
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				p := allPhases[(i+j)%len(allPhases)]
				switch (i + j) % 3 {
				case 0:
					app.SetPhase(p, func() {})
				case 1:
					app.RunPhase(p)
				case 2:
					app.PhaseSealed(p)
				}
			}
		}(i)
	}
	wg.Wait()
}
