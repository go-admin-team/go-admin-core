package runtime

import (
	"strings"
	"sync"
	"testing"

	"github.com/go-admin-team/go-admin-core/v2/logger"
)

// Acceptance 5: one callback panicking must not cost the others their turn,
// and must not take the process with it. Before this guard every registrant
// had to write its own recover, which means the protection was only as good as
// the least careful third-party module.
func TestPanicInOneCallbackDoesNotStopTheRest(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	var ran []string
	app.SetBefore(func() { ran = append(ran, "first") })
	app.SetBefore(func() { panic("boom") })
	app.SetBefore(func() { ran = append(ran, "third") })

	mustReturn(t, "RunBefore with a panicking callback", app.RunBefore)

	if got := strings.Join(ran, ","); got != "first,third" {
		t.Errorf("ran %q, want first,third", got)
	}
}

// The same guard covers the router registry, which is the one a third-party
// module actually registers into.
func TestPanicInOneRouterDoesNotStopTheRest(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	var ran []string
	app.SetAppRouters(func() { ran = append(ran, "first") })
	app.SetAppRouters(func() { panic("boom") })
	app.SetAppRouters(func() { ran = append(ran, "third") })

	app.RunAppRouters()

	if got := strings.Join(ran, ","); got != "first,third" {
		t.Errorf("ran %q, want first,third", got)
	}
}

// Acceptance 6: recovering without reporting would only move the silent
// failure. The line has to carry the panic value, the stack, and - because the
// stack unwinds into core, not into the module - where the callback was
// registered.
func TestPanicIsReportedWithSiteValueAndStack(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	app.SetBeforeWith(func() { panic("boom") }, WithName("crm-installer"))
	app.RunBefore()

	errors := rec.only(logger.ErrorLevel)
	if len(errors) != 1 {
		t.Fatalf("want exactly one error line, got %v", errors)
	}
	got := errors[0]
	for _, want := range []string{
		"crm-installer",             // the label the registrant chose
		"application_guard_test.go", // where it was registered
		"boom",                      // what the panic said
		"goroutine ",                // the stack, not just the message
		"TestPanicIsReportedWithSiteValueAndStack", // the frame that panicked
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the report must contain %q, got:\n%s", want, got)
		}
	}
}

// Without WithName the line still has to be actionable, which means the
// registration site is not optional.
func TestUnnamedPanicStillNamesTheRegistrationSite(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	app.SetAppRouters(func() { panic("boom") })
	app.RunAppRouters()

	errors := rec.only(logger.ErrorLevel)
	if len(errors) != 1 {
		t.Fatalf("want exactly one error line, got %v", errors)
	}
	if !strings.Contains(errors[0], "application_guard_test.go") {
		t.Errorf("the report must name the registration site, got:\n%s", errors[0])
	}
}

// Acceptance 8a: a callback that recovers for itself is not a special case.
// The panic stops inside it, the function returns normally, and the guard's
// recover sees nil - so nothing is logged twice. This falls out of Go's
// semantics; the test is here to keep it that way.
func TestCallbackThatRecoversForItselfIsNotReportedAgain(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	recovered := false
	app.SetBefore(func() {
		defer func() {
			if r := recover(); r != nil {
				recovered = true
			}
		}()
		panic("handled inside")
	})
	after := false
	app.SetBefore(func() { after = true })

	app.RunBefore()

	if !recovered {
		t.Error("the callback's own recover did not run")
	}
	if !after {
		t.Error("the following callback did not run")
	}
	if errors := rec.only(logger.ErrorLevel); len(errors) != 0 {
		t.Errorf("the framework must stay quiet when the callback handled it, got %v", errors)
	}
}

// Acceptance 8b: the boundary of the promise. recover only works on the
// goroutine that is panicking, so a callback whose real work is `go func()`
// - which is how go-admin-pro registers its jobs - is outside the guard
// entirely. Here the goroutine recovers for itself, as pro's does. Without
// that recover the process dies, and no amount of framework code changes it;
// TestGoroutinePanicEscapesTheGuard runs that case in a subprocess.
func TestGuardDoesNotReachIntoASpawnedGoroutine(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	var wg sync.WaitGroup
	wg.Add(1)
	handledInGoroutine := false
	app.SetBefore(func() {
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					handledInGoroutine = true
				}
			}()
			panic("boom in a goroutine")
		}()
	})

	app.RunBefore()
	wg.Wait()

	if !handledInGoroutine {
		t.Error("the goroutine's own recover did not run")
	}
	if errors := rec.only(logger.ErrorLevel); len(errors) != 0 {
		t.Errorf("the framework cannot see across a goroutine boundary, so it must log nothing, got %v", errors)
	}
}

// A nil logger is a bad configuration, but the paths that report a defect must
// not turn it into a second one: the panic still has to be reported somewhere
// rather than crashing on the way to being reported.
func TestGuardSurvivesANilLogger(t *testing.T) {
	app := NewConfig()
	previous := logger.DefaultLogger
	t.Cleanup(func() { logger.DefaultLogger = previous })
	app.SetLogger(nil)

	after := false
	app.SetBefore(func() { panic("boom") })
	app.SetBefore(func() { after = true })

	mustReturn(t, "RunBefore with a nil logger", app.RunBefore)

	if !after {
		t.Error("a nil logger must not stop the remaining callbacks")
	}
}
