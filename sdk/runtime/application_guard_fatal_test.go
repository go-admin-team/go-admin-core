package runtime

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/go-admin-team/go-admin-core/v2/logger"
)

// childEnv selects which scenario the re-executed test binary should play.
// Exiting the process cannot be asserted from inside the process that would
// exit, so the cases that really exit run in a child.
const childEnv = "GO_ADMIN_CORE_GUARD_CHILD"

// stdoutLogger writes where the parent can read it, and reports a level of the
// caller's choosing so a test can prove the exit does not depend on anything
// being logged.
type stdoutLogger struct{ level logger.Level }

func (l stdoutLogger) Init(...logger.Option) error                 { return nil }
func (l stdoutLogger) Options() logger.Options                     { return logger.Options{Level: l.level} }
func (l stdoutLogger) Fields(map[string]interface{}) logger.Logger { return l }
func (l stdoutLogger) String() string                              { return "stdout" }

func (l stdoutLogger) Log(_ logger.Level, v ...interface{}) {
	fmt.Fprintln(os.Stdout, v...)
}

func (l stdoutLogger) Logf(_ logger.Level, format string, v ...interface{}) {
	fmt.Fprintf(os.Stdout, format+"\n", v...)
}

// TestGuardChild is the body of every subprocess case. It is skipped in a
// normal run, and the parent selects a case through the environment.
func TestGuardChild(t *testing.T) {
	scenario := os.Getenv(childEnv)
	if scenario == "" {
		t.Skip("child of the guard subprocess tests; run through the parent")
	}

	app := NewConfig()
	switch scenario {
	case "fatal":
		app.SetLogger(stdoutLogger{level: logger.TraceLevel})
		app.SetBeforeWith(func() { panic("the database is not reachable") },
			WithFatal(), WithName("db-check"))
		app.SetBefore(func() { fmt.Println("MUST-NOT-RUN") })
		app.RunBefore()
		fmt.Println("MUST-NOT-REACH")

	case "fatal-silent-logger":
		// A logger that drops everything, including FatalLevel. This is the
		// reason the guard does not call Helper.Fatalf: that method returns
		// without exiting when the level filters it out, which would turn
		// "must not start" into "started anyway, quietly".
		app.SetLogger(stdoutLogger{level: logger.FatalLevel + 1})
		app.SetBeforeWith(func() { panic("boom") }, WithFatal())
		app.RunBefore()
		fmt.Println("MUST-NOT-REACH")

	case "goroutine":
		// The boundary of the guard, played out for real: the callback returns
		// cleanly and the panic happens later, on a goroutine no recover of
		// ours can reach.
		app.SetLogger(stdoutLogger{level: logger.TraceLevel})
		app.SetBefore(func() {
			done := make(chan struct{})
			go func() {
				close(done)
				panic("boom in a goroutine")
			}()
			<-done
		})
		app.RunBefore()
		select {} // let the goroutine bring the process down
	}
}

func runGuardChild(t *testing.T, scenario string) (string, int) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=^TestGuardChild$", "-test.v")
	cmd.Env = append(os.Environ(), childEnv+"="+scenario)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("the child exited successfully; it was supposed to die.\n%s", out)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("could not run the child: %v\n%s", err, out)
	}
	return string(out), exitErr.ExitCode()
}

// Acceptance 7: a callback marked fatal takes the process down, and says why
// on the way out. The exit code is part of it - a supervisor restarting the
// service reads that, not the log.
func TestFatalCallbackExitsTheProcess(t *testing.T) {
	out, code := runGuardChild(t, "fatal")

	if code != 1 {
		t.Errorf("the child exited with %d, want 1\n%s", code, out)
	}
	for _, want := range []string{"db-check", "the database is not reachable", "application_guard_fatal_test.go"} {
		if !strings.Contains(out, want) {
			t.Errorf("the child must say %q before exiting, got:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"MUST-NOT-RUN", "MUST-NOT-REACH"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("the process kept going after a fatal callback (%s):\n%s", unwanted, out)
		}
	}
}

// The exit must not be a side effect of logging. With a logger that filters
// everything out, nothing is printed and the process still has to die -
// otherwise raising the log level would silently re-enable a start-up that was
// declared impossible.
func TestFatalExitDoesNotDependOnTheLogLevel(t *testing.T) {
	out, code := runGuardChild(t, "fatal-silent-logger")

	if code != 1 {
		t.Errorf("the child exited with %d, want 1\n%s", code, out)
	}
	if strings.Contains(out, "MUST-NOT-REACH") {
		t.Errorf("a filtered log level let the process start anyway:\n%s", out)
	}
}

// The other half of acceptance 8: proof that the guard's boundary is real. A
// panic on a goroutine the callback started kills the process, and the guard
// never gets a word in. This is why the contract says the protection covers
// synchronous panics only - a module that copies pro's `go func()` shape is
// responsible for its own recover.
func TestGoroutinePanicEscapesTheGuard(t *testing.T) {
	out, code := runGuardChild(t, "goroutine")

	if code == 1 {
		t.Errorf("exit code 1 suggests the guard handled it; it cannot:\n%s", out)
	}
	if !strings.Contains(out, "panic: boom in a goroutine") {
		t.Errorf("the child must die of the raw panic, got:\n%s", out)
	}
	if strings.Contains(out, "continuing with the rest") {
		t.Errorf("the guard reported a panic it cannot actually see:\n%s", out)
	}
}

// The in-process half: the same decision, observed through the exit seam, so
// the branch is covered by the normal test run as well as by the subprocess.
//
// The stub returns where os.Exit would not, so execution continues past it -
// that is an artefact of the seam, not the behaviour under test.
func TestFatalCallbackCallsExitWithOne(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	previous := exit
	t.Cleanup(func() { exit = previous })
	var codes []int
	exit = func(code int) { codes = append(codes, code) }

	app.SetBeforeWith(func() { panic("boom") }, WithFatal())
	app.RunBefore()

	if len(codes) != 1 || codes[0] != 1 {
		t.Errorf("exit was called with %v, want exactly one call with 1", codes)
	}
}

// A callback that does not declare itself fatal must never exit, whatever it
// panics with. The default is the degradable one; that is what makes WithFatal
// meaningful.
func TestDefaultCallbackNeverExits(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	previous := exit
	t.Cleanup(func() { exit = previous })
	called := false
	exit = func(int) { called = true }

	app.SetBefore(func() { panic("boom") })
	app.RunBefore()

	if called {
		t.Error("a callback registered without WithFatal must not bring the process down")
	}
}
