package runtime

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/v2/logger"
)

// The window this closes: a configuration reload that started before the
// shutdown did finishes by rebuilding the resources and announcing them. In
// the middle of a shutdown that undoes the cleanup which has just run and
// starts consumers on a process being taken apart.
func TestAReloadArrivingDuringShutdownRunsNothing(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	ran := 0
	app.SetPhase(AfterResource, func() { ran++ })
	app.RunPhase(AfterResource)

	app.BeginShutdown()
	app.RunPhase(AfterResource)

	if ran != 1 {
		t.Errorf("the AfterResource callback ran %d times, want 1: the reload during shutdown was let through", ran)
	}
	warns := rec.only(logger.WarnLevel)
	if len(warns) != 1 {
		t.Fatalf("want one warning about the refused trigger, got %v", warns)
	}
	if !strings.Contains(warns[0], "AfterResource") || !strings.Contains(warns[0], "shutting down") {
		t.Errorf("the warning must name the phase and the reason, got:\n%s", warns[0])
	}
}

// The other half: AfterResource never closes, so without the flag anything
// could keep registering into it while the process goes down, to be run by
// the next reload.
func TestRegisteringDuringShutdownIsIgnoredAndReported(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	app.BeginShutdown()

	ran := false
	app.SetPhase(AfterResource, func() { ran = true })
	app.RunPhase(AfterResource)

	if ran {
		t.Error("a callback registered during shutdown ran anyway")
	}
	// The callback must not even be kept. AfterResource never seals, so
	// without this gate anything could keep appending to a registry that is
	// never walked again - the leak the growth warning cannot help with,
	// because there are no further rounds to compare.
	i, _ := AfterResource.index()
	app.mux.RLock()
	kept := len(app.phases[i].entries)
	app.mux.RUnlock()
	if kept != 0 {
		t.Errorf("the registry kept %d callbacks registered during shutdown, want 0", kept)
	}
	warns := rec.only(logger.WarnLevel)
	if len(warns) == 0 {
		t.Fatal("dropping the registration must be reported")
	}
	for _, want := range []string{"AfterResource", "shutting down", "shutdown_gate_test.go"} {
		if !strings.Contains(warns[0], want) {
			t.Errorf("the warning must contain %q, got:\n%s", want, warns[0])
		}
	}
}

// The start-up phases are closed by it as well: an application on its way out
// has no use for a router hook.
func TestTheStartUpPhasesAreClosedByShutdown(t *testing.T) {
	for _, p := range []Phase{AfterResource, BeforeRouter, AfterListen} {
		app := NewConfig()
		captureLogs(t, app)
		app.BeginShutdown()

		ran := false
		app.SetPhase(p, func() { ran = true })
		app.RunPhase(p)

		if ran {
			t.Errorf("%v ran during shutdown", p)
		}
	}
}

// BeforeExit is exempt, and this is the case that matters most: the flag is
// set at the start of the shutdown path, so gating every phase alike would
// make it skip the cleanup it was added to protect.
func TestBeforeExitStillWorksAfterBeginShutdown(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	app.BeginShutdown()

	var ran []string
	app.SetShutdown(func(context.Context) { ran = append(ran, "shutdown-hook") })
	app.SetPhase(BeforeExit, func() { ran = append(ran, "phase-hook") })

	if err := app.RunShutdown(context.Background()); err != nil {
		t.Fatalf("RunShutdown returned %v, want nil", err)
	}
	if got := strings.Join(ran, ","); got != "phase-hook,shutdown-hook" {
		t.Errorf("ran %q, want phase-hook,shutdown-hook: cleanup registered after BeginShutdown must still run", got)
	}
}

// The same through the phase spelling of the call, which is the one that
// would silently do nothing if the gate did not exempt BeforeExit.
func TestRunPhaseBeforeExitStillWorksAfterBeginShutdown(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	ran := false
	app.SetPhase(BeforeExit, func() { ran = true })
	app.BeginShutdown()
	app.RunPhase(BeforeExit)

	if !ran {
		t.Error("RunPhase(BeforeExit) did nothing after BeginShutdown")
	}
}

// A host that only calls RunShutdown is covered from that moment on, without
// having to know the flag exists.
func TestRunShutdownClosesTheStartUpPhasesToo(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	ran := 0
	app.SetPhase(AfterResource, func() { ran++ })
	if err := app.RunShutdown(context.Background()); err != nil {
		t.Fatalf("RunShutdown returned %v, want nil", err)
	}

	app.RunPhase(AfterResource)
	if ran != 0 {
		t.Errorf("AfterResource ran %d times after RunShutdown, want 0", ran)
	}
}

// It closes the door, it does not reach into the room: a round already
// running finishes. Interrupting it would leave the resources half-attached,
// which is worse than one more round of hooks that are meant to be repeatable.
func TestBeginShutdownDoesNotInterruptARoundAlreadyRunning(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	started := make(chan struct{})
	release := make(chan struct{})
	secondRan := make(chan struct{})

	// Two callbacks, because the claim is about the rest of the round: the
	// flag is set while the first one is still inside the batch, and the
	// second one has to run anyway. Stopping half way through would leave
	// the resources half-attached, which is worse than one more round of
	// hooks that are meant to be repeatable.
	app.SetPhase(AfterResource, func() {
		close(started)
		<-release
	})
	app.SetPhase(AfterResource, func() { close(secondRan) })

	go app.RunPhase(AfterResource)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("the round never started")
	}

	app.BeginShutdown()
	close(release)

	select {
	case <-secondRan:
	case <-time.After(2 * time.Second):
		t.Fatal("BeginShutdown cut short a round that was already running")
	}
}

// A process does not un-shut-down, so the call is one-way and saying it twice
// changes nothing.
func TestBeginShutdownIsIdempotent(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	app.BeginShutdown()
	app.BeginShutdown()

	ran := false
	app.SetPhase(AfterResource, func() { ran = true })
	app.RunPhase(AfterResource)
	if ran {
		t.Error("the phase reopened")
	}
}

// The zero value has to work here too: the flag is an atomic that nothing
// initialises, which is the point of choosing one.
func TestBeginShutdownOnAZeroValueApplication(t *testing.T) {
	app := &Application{}
	captureLogs(t, app)

	mustReturn(t, "BeginShutdown on a zero-value Application", app.BeginShutdown)

	ran := false
	app.SetPhase(AfterResource, func() { ran = true })
	app.RunPhase(AfterResource)
	if ran {
		t.Error("the gate did not hold on a zero-value Application")
	}
}

// The flag is read on paths that hold no lock and written from the signal
// handler's goroutine, which is what an atomic is for.
func TestTheShutdownGateIsRaceFree(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				switch (i + j) % 4 {
				case 0:
					app.SetPhase(AfterResource, func() {})
				case 1:
					app.RunPhase(AfterResource)
				case 2:
					app.PhaseSealed(AfterResource)
				case 3:
					if i == 7 {
						app.BeginShutdown()
					}
				}
			}
		}(i)
	}
	wg.Wait()
}
