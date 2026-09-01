package runtime

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-admin-team/go-admin-core/v2/logger"
	"github.com/go-admin-team/go-admin-core/v2/storage"
	"gorm.io/gorm"
)

// recordingLogger keeps every line so a test can assert on what was said.
//
// It reports TraceLevel so nothing is filtered out: the guard's own defence
// against Helper.Fatalf is that level filtering must never decide whether
// something is reported, and a test logger that filters would hide exactly the
// regression this file exists to catch.
type recordingLogger struct {
	mu    sync.Mutex
	lines []string
}

func (l *recordingLogger) Init(...logger.Option) error { return nil }
func (l *recordingLogger) Options() logger.Options     { return logger.Options{Level: logger.TraceLevel} }
func (l *recordingLogger) Fields(map[string]interface{}) logger.Logger {
	return l
}
func (l *recordingLogger) String() string { return "recording" }

func (l *recordingLogger) Log(level logger.Level, v ...interface{}) {
	l.record(level, fmt.Sprint(v...))
}

func (l *recordingLogger) Logf(level logger.Level, format string, v ...interface{}) {
	l.record(level, fmt.Sprintf(format, v...))
}

func (l *recordingLogger) record(level logger.Level, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, level.String()+" "+msg)
}

func (l *recordingLogger) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.lines...)
}

// only returns the lines logged at one level.
func (l *recordingLogger) only(level logger.Level) []string {
	prefix := level.String() + " "
	var out []string
	for _, line := range l.all() {
		if strings.HasPrefix(line, prefix) {
			out = append(out, line)
		}
	}
	return out
}

// captureLogs points the application at a recording logger for one test.
//
// SetLogger writes the package-level logger.DefaultLogger, so this is
// process-wide: no test that calls it may run in parallel.
func captureLogs(t *testing.T, app *Application) *recordingLogger {
	t.Helper()
	rec := &recordingLogger{}
	previous := logger.DefaultLogger
	t.Cleanup(func() { logger.DefaultLogger = previous })
	app.SetLogger(rec)
	return rec
}

// Acceptance 1: RunBefore runs every registered callback, in the order they
// were registered. The order is part of the contract - it is what lets a
// module rely on an earlier one having run - so it is asserted, not assumed.
func TestRunBeforeRunsInRegistrationOrder(t *testing.T) {
	app := NewConfig()
	var order []string
	for _, name := range []string{"first", "second", "third"} {
		app.SetBefore(func() { order = append(order, name) })
	}

	app.RunBefore()

	if got := strings.Join(order, ","); got != "first,second,third" {
		t.Errorf("ran %q, want first,second,third", got)
	}
}

// Acceptance 2: the same promise for the router registry.
func TestRunAppRoutersRunsInRegistrationOrder(t *testing.T) {
	app := NewConfig()
	var order []string
	for _, name := range []string{"first", "second", "third"} {
		app.SetAppRouters(func() { order = append(order, name) })
	}

	app.RunAppRouters()

	if got := strings.Join(order, ","); got != "first,second,third" {
		t.Errorf("ran %q, want first,second,third", got)
	}
}

// Acceptance 3: the pre-existing way of using the registry - ask for the
// callbacks and run them yourself - keeps working unchanged. go-admin-pro does
// exactly this, and it is not being changed in this batch.
func TestSelfWrittenLoopStillWorks(t *testing.T) {
	app := NewConfig()
	var ran []string
	app.SetBefore(func() { ran = append(ran, "before-1") })
	app.SetBefore(func() { ran = append(ran, "before-2") })
	app.SetAppRouters(func() { ran = append(ran, "router-1") })

	for _, f := range app.GetBefore() {
		f()
	}
	for _, f := range app.GetAppRouters() {
		f()
	}

	if got := strings.Join(ran, ","); got != "before-1,before-2,router-1" {
		t.Errorf("ran %q, want before-1,before-2,router-1", got)
	}
}

// Get* hands out a copy. The caller ranges over the result with no lock held,
// so returning the live slice was the same defect GetAllDb was fixed for.
func TestGetReturnsASnapshot(t *testing.T) {
	app := NewConfig()
	app.SetBefore(func() {})

	snapshot := app.GetBefore()
	snapshot[0] = nil

	if got := app.GetBefore(); got[0] == nil {
		t.Error("writing to the returned slice reached the registry; Get* must return a copy")
	}
}

// Acceptance 4: Run* is idempotent. Something has to be, because a fork that
// added its own call keeps it while the framework gains one too, and a startup
// hook that runs twice is a class of bug that only shows up in production.
func TestRunIsIdempotent(t *testing.T) {
	app := NewConfig()
	before, routers := 0, 0
	app.SetBefore(func() { before++ })
	app.SetAppRouters(func() { routers++ })

	for i := 0; i < 3; i++ {
		app.RunBefore()
		app.RunAppRouters()
	}

	if before != 1 || routers != 1 {
		t.Errorf("before ran %d times and appRouters %d, want 1 each", before, routers)
	}
}

// Acceptance 9: a registration that arrives after the registry has run is
// dropped and said out loud. Keeping it would be worse: it would sit in a
// slice nobody walks again, which is the silent failure this batch exists to
// remove.
func TestRegistrationAfterRunIsDroppedAndReported(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	ran := 0
	app.SetBefore(func() { ran++ })
	app.RunBefore()

	late := 0
	app.SetBefore(func() { late++ })
	app.RunBefore()

	if late != 0 {
		t.Errorf("the late callback ran %d times, want 0", late)
	}
	if ran != 1 {
		t.Errorf("the early callback ran %d times, want 1", ran)
	}
	if got := len(app.GetBefore()); got != 1 {
		t.Errorf("the registry holds %d callbacks, want 1", got)
	}

	errors := rec.only(logger.ErrorLevel)
	if len(errors) != 1 {
		t.Fatalf("want exactly one error line, got %v", errors)
	}
	// What happened, why, and where the offending registration is: a line
	// missing any of the three leaves the reader with nothing to act on.
	for _, want := range []string{"SetBefore", "ignored", "RunBefore", "application_registry_test.go"} {
		if !strings.Contains(errors[0], want) {
			t.Errorf("the error must mention %q, got: %s", want, errors[0])
		}
	}
}

// The registries close one at a time. They have separate entry points, so
// running one says nothing about whether the other is still open.
func TestSealingIsPerRegistry(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	app.RunBefore()
	if !app.BeforeSealed() {
		t.Error("RunBefore must close the before registry")
	}
	if app.AppRoutersSealed() {
		t.Error("RunBefore must not close the appRouters registry")
	}

	ran := false
	app.SetAppRouters(func() { ran = true })
	app.RunAppRouters()
	if !ran {
		t.Error("a router registered after RunBefore must still run")
	}
	if !app.AppRoutersSealed() {
		t.Error("RunAppRouters must close the appRouters registry")
	}
}

// labelledCache and labelledQueue carry an id so a test can tell which
// instance the getters hand back; the real memory adapters both report their
// type, which would pass whether or not the field was replaced.
type labelledCache struct {
	storage.AdapterCache
	id string
}

func (c *labelledCache) String() string { return c.id }

type labelledQueue struct {
	storage.AdapterQueue
	id string
}

func (q *labelledQueue) String() string { return q.id }

// Acceptance 10: closing the registries must not reach the resource fields.
// Reloading the configuration re-runs the setup callbacks while requests are
// being served - replacing the database, the cache and the queue - so freezing
// those would turn a config change into a silent no-op.
func TestSealingDoesNotFreezeResources(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	app.RunBefore()
	app.RunAppRouters()

	db := &gorm.DB{}
	app.SetDbByTenant("tenant-a", db)
	if got := app.GetDbByTenant("tenant-a"); got != db {
		t.Error("SetDbByTenant stopped working once the registries were closed")
	}

	app.SetCacheAdapter(&labelledCache{id: "cache-after-seal"})
	if got := app.GetCacheAdapter().String(); got != "cache-after-seal" {
		t.Errorf("GetCacheAdapter returned %q, want the adapter set after sealing", got)
	}

	app.SetQueueAdapter(&labelledQueue{id: "queue-after-seal"})
	if got := app.GetQueueAdapter().String(); got != "queue-after-seal" {
		t.Errorf("GetQueueAdapter returned %q, want the adapter set after sealing", got)
	}

	app.SetConfigValue("key", "value")
	if got := app.GetConfigValue("key"); got != "value" {
		t.Errorf("GetConfigValue returned %v, want the value set after sealing", got)
	}
}

// A callback reaching back into Application is the normal case, not an edge
// one: app/demo/router/router.go asks for the engine and sets it when there is
// none, and that function is exactly what RunAppRouters executes. sync.RWMutex
// is not reentrant, so running callbacks under the lock hangs here - which is
// why the guard is a timeout rather than a plain call.
func TestCallbackMayCallBackIntoRuntime(t *testing.T) {
	app := NewConfig()
	gin.SetMode(gin.TestMode)

	var seen http.Handler
	mustReturn(t, "RunAppRouters with a callback that touches Application", func() {
		app.SetAppRouters(func() {
			if h := app.GetEngine(); h == nil {
				app.SetEngine(gin.New())
			}
			seen = app.GetEngine()
			app.GetRouter()
			app.GetDefaultTenant()
		})
		app.RunAppRouters()
	})

	if seen == nil {
		t.Error("the callback did not observe the engine it had just set")
	}
}

// Mixing the two styles runs the callbacks twice, and core cannot prevent that
// - it never sees a loop written downstream. What it can do is say so, which
// is the difference between a startup that is wrong and one that is wrong and
// undiagnosable.
func TestMixingGetAndRunWarns(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	ran := 0
	app.SetBefore(func() { ran++ })

	for _, f := range app.GetBefore() {
		f()
	}
	app.RunBefore()

	if ran != 2 {
		t.Errorf("the callback ran %d times, want 2 (the double run the warning is about)", ran)
	}
	warnings := rec.only(logger.WarnLevel)
	if len(warnings) != 1 {
		t.Fatalf("want exactly one warning, got %v", warnings)
	}
	for _, want := range []string{"GetBefore", "RunBefore", "twice"} {
		if !strings.Contains(warnings[0], want) {
			t.Errorf("the warning must mention %q, got: %s", want, warnings[0])
		}
	}
}

// The warning belongs to a run that has something to run. A no-op second call
// cannot double-run anything, and repeating the warning on every call would
// train readers to ignore it.
func TestNoWarningWhenThereIsNothingToRun(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	app.GetBefore()
	app.RunBefore()

	if warnings := rec.only(logger.WarnLevel); len(warnings) != 0 {
		t.Errorf("an empty registry must not warn, got %v", warnings)
	}
}

// The options carry the failure level, so an unnamed, unoptioned registration
// has to end up on the same path with the same defaults - otherwise "no
// options means the old behaviour" is a promise held up by nothing.
func TestWithOptionsSharesThePlainPath(t *testing.T) {
	app := NewConfig()
	var order []string
	app.SetBefore(func() { order = append(order, "plain") })
	app.SetBeforeWith(func() { order = append(order, "with-name") }, WithName("named"))
	app.SetAppRouters(func() { order = append(order, "router-plain") })
	app.SetAppRoutersWith(func() { order = append(order, "router-named") }, WithName("named"))

	app.RunBefore()
	app.RunAppRouters()

	if got := strings.Join(order, ","); got != "plain,with-name,router-plain,router-named" {
		t.Errorf("ran %q, want plain,with-name,router-plain,router-named", got)
	}
}

// The empty case used to return the nil field itself. Handing back an empty
// non-nil slice instead would be a visible change for anyone who compares
// against nil, and there is nothing to gain from it.
func TestGetOnAnEmptyRegistryReturnsNil(t *testing.T) {
	app := NewConfig()

	if got := app.GetBefore(); got != nil {
		t.Errorf("GetBefore returned %#v, want nil", got)
	}
	if got := app.GetAppRouters(); got != nil {
		t.Errorf("GetAppRouters returned %#v, want nil", got)
	}
}
