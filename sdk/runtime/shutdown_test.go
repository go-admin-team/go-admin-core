package runtime

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/v2/logger"
	"gorm.io/gorm"
)

// Cleanup unwinds: the callback registered last is the one most likely to
// depend on the others, so it goes first. SetShutdown and SetPhase(BeforeExit)
// register into the same phase, so one pass runs both in one order.
func TestShutdownCallbacksRunInReverseRegistrationOrder(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	var ran []string
	app.SetShutdown(func(context.Context) { ran = append(ran, "first") })
	app.SetPhase(BeforeExit, func() { ran = append(ran, "second") })
	app.SetShutdown(func(context.Context) { ran = append(ran, "third") })

	if err := app.RunShutdown(context.Background()); err != nil {
		t.Fatalf("RunShutdown returned %v, want nil", err)
	}
	if got := strings.Join(ran, ","); got != "third,second,first" {
		t.Errorf("ran %q, want third,second,first", got)
	}
}

// The context is the whole reason for the second signature: a callback that
// can shorten its work needs to see how much of the budget is left.
func TestShutdownCallbacksAreGivenTheContext(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var got context.Context
	app.SetShutdown(func(c context.Context) { got = c })

	if err := app.RunShutdown(ctx); err != nil {
		t.Fatalf("RunShutdown returned %v, want nil", err)
	}
	if got == nil {
		t.Fatal("the callback was not given a context")
	}
	if _, ok := got.Deadline(); !ok {
		t.Error("the callback's context carries no deadline, so it cannot see the budget")
	}
}

// Cleanup must not run twice. Closing a pool a second time is worse than not
// closing it, which is why this phase is taken and closed in one critical
// section rather than re-read on every call.
func TestRunShutdownTwiceDoesNotRunCleanupTwice(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	calls := 0
	app.SetShutdown(func(context.Context) { calls++ })

	for i := 0; i < 3; i++ {
		if err := app.RunShutdown(context.Background()); err != nil {
			t.Fatalf("RunShutdown returned %v, want nil", err)
		}
	}
	if calls != 1 {
		t.Errorf("the cleanup ran %d times, want 1", calls)
	}
}

// Concurrent callers must not each get a copy of the batch either.
func TestConcurrentRunShutdownRunsTheCleanupOnce(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	var mu sync.Mutex
	calls := 0
	for i := 0; i < 3; i++ {
		app.SetShutdown(func(context.Context) {
			mu.Lock()
			defer mu.Unlock()
			calls++
		})
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = app.RunShutdown(context.Background())
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if calls != 3 {
		t.Errorf("three callbacks ran %d times in total across eight concurrent calls, want 3", calls)
	}
}

// Registering after the phase has run is refused and reported: the callback
// would otherwise sit in a slice nobody walks again, and the thing it was
// meant to clean up would never be cleaned up.
func TestSetShutdownAfterTheRunIsRefusedAndReported(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	_ = app.RunShutdown(context.Background())

	late := false
	app.SetShutdown(func(context.Context) { late = true })
	_ = app.RunShutdown(context.Background())

	if late {
		t.Error("a callback registered after RunShutdown ran anyway")
	}
	errors := rec.only(logger.ErrorLevel)
	if len(errors) != 1 {
		t.Fatalf("want one error line for the late registration, got %v", errors)
	}
	for _, want := range []string{"SetShutdown", "RunShutdown", "shutdown_test.go"} {
		if !strings.Contains(errors[0], want) {
			t.Errorf("the report must name %q, got:\n%s", want, errors[0])
		}
	}
}

// When the budget runs out the wait ends and the work does not. The caller
// gets ctx.Err() promptly, the callback that is holding things up is named,
// and it carries on - which is the honest description of what Go can do to a
// function that does not check its context.
func TestRunShutdownStopsWaitingWhenTheBudgetRunsOut(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	release := make(chan struct{})
	finished := make(chan struct{})
	ran := make(chan string, 2)

	app.SetShutdown(func(context.Context) { ran <- "registered-first" })
	app.SetShutdown(func(context.Context) {
		ran <- "the-slow-one"
		<-release
		close(finished)
	}, WithName("slow-drain"))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	returned := make(chan error, 1)
	go func() { returned <- app.RunShutdown(ctx) }()

	var err error
	select {
	case err = <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("RunShutdown did not return when its budget ran out")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("RunShutdown returned %v, want context.DeadlineExceeded", err)
	}

	reports := rec.only(logger.ErrorLevel)
	if len(reports) != 1 {
		t.Fatalf("want one report naming the callback that ran out the budget, got %v", reports)
	}
	for _, want := range []string{"slow-drain", "shutdown_test.go", "budget"} {
		if !strings.Contains(reports[0], want) {
			t.Errorf("the report must contain %q, got:\n%s", want, reports[0])
		}
	}

	// What was abandoned is the wait. The callback is still there, and it
	// finishes - along with the ones behind it - if it ever gets going.
	close(release)
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("the abandoned callback did not carry on")
	}
	select {
	case <-time.After(2 * time.Second):
		t.Fatal("the callbacks behind the slow one never ran")
	case got := <-ran:
		if got != "the-slow-one" {
			t.Errorf("the first callback to run was %q, want the-slow-one (reverse order)", got)
		}
	}
}

// With nothing registered there is nothing to wait for, so there is no
// goroutine, the context is never consulted, and an expired budget is not
// reported as a timeout for work that does not exist.
func TestRunShutdownWithNothingRegisteredIsNotATimeout(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // the budget is gone before the call

	if err := app.RunShutdown(ctx); err != nil {
		t.Errorf("RunShutdown with nothing registered returned %v, want nil", err)
	}
}

// WithFatal is stripped here and reported. Exiting from inside cleanup skips
// every callback after it - the opposite of what the option is for - but
// refusing the registration would mean one mistyped option silently costs
// the whole cleanup.
func TestWithFatalOnAShutdownCallbackIsStrippedAndReported(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	previous := exit
	t.Cleanup(func() { exit = previous })
	exited := false
	exit = func(int) { exited = true }

	after := false
	app.SetShutdown(func(context.Context) { after = true })
	app.SetShutdown(func(context.Context) { panic("boom") }, WithFatal(), WithName("pool-close"))

	registration := rec.only(logger.ErrorLevel)
	if len(registration) != 1 {
		t.Fatalf("want one report at the registration site, got %v", registration)
	}
	for _, want := range []string{"WithFatal", "pool-close", "shutdown_test.go"} {
		if !strings.Contains(registration[0], want) {
			t.Errorf("the report must contain %q, got:\n%s", want, registration[0])
		}
	}

	if err := app.RunShutdown(context.Background()); err != nil {
		t.Fatalf("RunShutdown returned %v, want nil", err)
	}
	if exited {
		t.Error("a shutdown callback marked WithFatal exited the process anyway")
	}
	if !after {
		t.Error("the cleanup registered before the fatal one never ran")
	}

	// The option is dropped, the callback is not: refusing the registration
	// would mean one mistyped option costs the cleanup entirely, and nothing
	// afterwards would ever say so.
	reports := rec.only(logger.ErrorLevel)
	if len(reports) != 2 {
		t.Fatalf("want the registration report and the panic report, got %v", reports)
	}
	for _, want := range []string{"panicked", "pool-close"} {
		if !strings.Contains(reports[1], want) {
			t.Errorf("the callback was kept, so its panic must be reported with %q, got:\n%s", want, reports[1])
		}
	}
}

// The guard covers this phase like the others: one callback panicking does
// not cost the rest.
func TestPanicInOneShutdownCallbackDoesNotStopTheRest(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	var ran []string
	app.SetShutdown(func(context.Context) { ran = append(ran, "first") })
	app.SetShutdown(func(context.Context) { panic("boom") })
	app.SetShutdown(func(context.Context) { ran = append(ran, "third") })

	if err := app.RunShutdown(context.Background()); err != nil {
		t.Fatalf("RunShutdown returned %v, want nil", err)
	}
	if got := strings.Join(ran, ","); got != "third,first" {
		t.Errorf("ran %q, want third,first", got)
	}
	if reports := rec.only(logger.ErrorLevel); len(reports) != 1 {
		t.Errorf("want one panic report, got %v", reports)
	}
}

// Cleanup calls back into Application - a hook that closes the database asks
// for it first - and sync.RWMutex is not reentrant.
func TestShutdownCallbacksRunWithNoLockHeld(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	reached := false
	app.SetShutdown(func(context.Context) {
		app.SetDb(&gorm.DB{})
		app.GetDb()
		app.PhaseSealed(BeforeExit)
		reached = true
	})

	mustReturn(t, "RunShutdown with a callback that calls back in", func() {
		_ = app.RunShutdown(context.Background())
	})
	if !reached {
		t.Error("the callback did not get through calling back into Application")
	}
}

// A nil context is a bug in the caller, but these callbacks are the cleanup:
// panicking to report the bug would skip all of them.
func TestRunShutdownWithANilContextRunsTheCleanup(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	ran := false
	app.SetShutdown(func(context.Context) { ran = true })

	mustReturn(t, "RunShutdown(nil)", func() {
		//lint:ignore SA1012 passing nil is the caller mistake under test
		_ = app.RunShutdown(nil)
	})
	if !ran {
		t.Error("a nil context cost the cleanup")
	}
}

// The in-process test above uses the exit seam, which returns where os.Exit
// would not. This one is the real thing: a process that runs a panicking
// WithFatal cleanup callback has to survive it, run the rest of the cleanup,
// and leave with status 0.
func TestShutdownFatalDoesNotTakeTheProcessDown(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestGuardChild$", "-test.v")
	cmd.Env = append(os.Environ(), childEnv+"=shutdown-fatal")
	out, err := cmd.CombinedOutput()

	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("could not run the child: %v\n%s", err, out)
		}
		t.Fatalf("the child exited with %d; a fatal cleanup callback must not take the process down\n%s",
			exitErr.ExitCode(), out)
	}
	for _, want := range []string{"WithFatal ignored", "CLEANUP-AFTER-THE-PANIC", "REACHED-THE-END"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the child must print %q, got:\n%s", want, out)
		}
	}
}
