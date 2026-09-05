package bootstrap

import (
	"testing"

	coreconfig "github.com/go-admin-team/go-admin-core/v2/config"
	"github.com/go-admin-team/go-admin-core/v2/config/source"
	"github.com/go-admin-team/go-admin-core/v2/config/source/memory"
	"github.com/go-admin-team/go-admin-core/v2/sdk"
	"github.com/go-admin-team/go-admin-core/v2/sdk/config"
	"github.com/go-admin-team/go-admin-core/v2/sdk/runtime"
)

// The smallest settings tree Setup will accept: it runs the logger setup and
// the multi-database fill on the way in, and both read from this.
const settings = `settings:
  application:
    host: 0.0.0.0
    port: 8000
    name: bootstrap-test
    mode: dev
  logger:
    path: ""
    stdout: ""
    level: error
  jwt:
    secret: test
    timeout: 3600
  database:
    driver: sqlite3
    source: ":memory:"
`

// isolate gives one test its own runtime and puts back the process-wide
// config object afterwards. Setup replaces both, so none of these tests may
// run in parallel.
//
// The config object is not Closed, deliberately. Close makes the watcher
// goroutine log "watcher stopped" through logger.DefaultLogger on its way
// out, and the next Setup in this binary overwrites that same package
// variable from the test goroutine - an unsynchronised global that -race
// reports, and that this test would be creating rather than finding. Left
// alone, the watcher stays parked in Next() on a memory source nobody
// updates: one goroutine per test, which ends with the binary and cannot
// interfere with anything.
//
// The underlying defect is real but not this commit's: Logger.Setup writes
// logger.DefaultLogger on every configuration reload, from the watcher
// goroutine, while everything else in the process reads it.
func isolate(t *testing.T) {
	t.Helper()

	previousRuntime := sdk.Runtime
	previousConfig := coreconfig.DefaultConfig
	t.Cleanup(func() {
		coreconfig.DefaultConfig = previousConfig
		sdk.Runtime = previousRuntime
	})
	sdk.Runtime = runtime.NewConfig()
}

func newSource() source.Source {
	return memory.NewSource(memory.WithYAML([]byte(settings)))
}

// The announcement goes last. AfterResource means "the database, the cache
// and the queue are ready", so a hook that runs before the callbacks that
// build them would be handed the previous set - or nothing at all on first
// start. This is the ordering the function exists to own.
func TestTheResourcePhaseIsAnnouncedAfterEveryCallback(t *testing.T) {
	isolate(t)

	var order []string
	sdk.Runtime.SetPhase(runtime.AfterResource, func() { order = append(order, "phase") })

	SetupConfig(newSource(),
		func() { order = append(order, "first") },
		func() { order = append(order, "second") },
	)

	want := []string{"first", "second", "phase"}
	if len(order) != len(want) {
		t.Fatalf("ran %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("ran %v, want %v", order, want)
		}
	}
}

// With no callbacks at all the phase is still announced: an application whose
// resources are all built by AfterResource hooks is a legitimate arrangement,
// and it would get nothing if the announcement only came with company.
func TestTheResourcePhaseIsAnnouncedWithNoCallbacks(t *testing.T) {
	isolate(t)

	ran := false
	sdk.Runtime.SetPhase(runtime.AfterResource, func() { ran = true })

	SetupConfig(newSource())

	if !ran {
		t.Error("SetupConfig with no callbacks announced nothing")
	}
}

// config.Setup stays inert, and that is the point of having two functions.
// Three of the four call sites in a go-admin tree are CLI commands - migrate,
// config - and starting queue consumers or warming a cache on the way to
// dumping a config file is the wrong thing to do quietly.
func TestConfigSetupOnItsOwnAnnouncesNothing(t *testing.T) {
	isolate(t)

	ran := false
	sdk.Runtime.SetPhase(runtime.AfterResource, func() { ran = true })

	config.Setup(newSource(), func() {})

	if ran {
		t.Error("config.Setup announced the resource phase; only SetupConfig may")
	}
}

// The announcement follows sdk.Runtime as it is when the callbacks run, not
// as it was when SetupConfig was called. Anything else would leave a process
// that swaps the runtime announcing into the one it replaced.
func TestTheAnnouncementFollowsTheCurrentRuntime(t *testing.T) {
	isolate(t)

	replaced := runtime.NewConfig()
	ran := false
	replaced.SetPhase(runtime.AfterResource, func() { ran = true })

	SetupConfig(newSource(), func() { sdk.Runtime = replaced })

	if !ran {
		t.Error("the announcement went to the runtime that was current at wiring time, not at run time")
	}
}

// fs may share its backing array with a slice the caller still holds, and
// appending to it in place would write the trigger into their spare capacity
// - visible to them as a callback they never registered.
func TestTheCallersSliceIsNotWrittenTo(t *testing.T) {
	isolate(t)

	callers := make([]func(), 1, 4) // room to spare, which is what makes it possible
	callers[0] = func() {}

	SetupConfig(newSource(), callers...)

	if got := callers[:cap(callers)]; got[1] != nil || got[2] != nil || got[3] != nil {
		t.Error("SetupConfig wrote into the caller's spare capacity")
	}
}
