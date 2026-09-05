package config

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/v2/config/source"
	"github.com/go-admin-team/go-admin-core/v2/config/source/memory"
)

// reloadProbe is a config Entity whose OnChange does what every real reload
// hook does first: read back the configuration it was just told had changed.
// It reports entering and returning separately, so a test can tell "the hook
// never ran" apart from "the hook ran and never came back".
//
// It reads through the Config instance rather than the package-level
// DefaultConfig on purpose: Get/Map/Scan are the same methods either way, and
// the watcher goroutine can outlive a test's cleanup by a moment, which would
// make touching a package-level variable from inside the hook a race of the
// test's own making.
type reloadProbe struct {
	mu  sync.Mutex
	cfg Config

	entered  chan struct{}
	returned chan struct{}
	observed chan string
}

func newReloadProbe() *reloadProbe {
	return &reloadProbe{
		entered:  make(chan struct{}, 1),
		returned: make(chan struct{}, 1),
		observed: make(chan string, 8),
	}
}

func (p *reloadProbe) attach(c Config) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cfg = c
}

func (p *reloadProbe) config() Config {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cfg
}

func (p *reloadProbe) OnChange() {
	signal(p.entered)

	c := p.config()
	if c != nil {
		// All three read entry points, because all three take RLock.
		_ = c.Map()
		var dst map[string]interface{}
		_ = c.Scan(&dst)
		signal2(p.observed, c.Get("key").String(""))
	}

	signal(p.returned)
}

// Non-blocking so a hook that fires more often than a test looks never blocks
// the watcher goroutine on a full channel.
func signal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func signal2(ch chan string, v string) {
	select {
	case ch <- v:
	default:
	}
}

// The reload path. This is the one that was broken: the watcher held the
// config write lock across OnChange (config/default.go run), Get/Map/Scan take
// RLock, and sync.RWMutex is not reentrant - so a hook that read the
// configuration blocked against a lock held by its own goroutine. The watcher
// then stayed blocked for the life of the process and every later change was
// dropped without a word.
//
// With the hook inside the lock this test fails on the "did not return"
// branch; with it outside, it passes.
func TestOnChangeCanReadTheConfigOnReload(t *testing.T) {
	src := memory.NewSource(memory.WithJSON([]byte(`{"key": "v0"}`)))

	probe := newReloadProbe()
	conf, err := NewConfig(WithSource(src), WithEntity(probe))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	t.Cleanup(func() { _ = conf.Close() })
	probe.attach(conf)

	// The watcher registers itself from a goroutine NewConfig starts, so a
	// single write can land before there is anything listening for it and be
	// dropped. Keep writing until the hook is entered rather than sleeping
	// and hoping.
	if !writeUntil(t, src, probe.entered, 3*time.Second) {
		t.Fatal("OnChange was never called within 3s: the reload never reached the entity, so this test proves nothing about the lock")
	}

	select {
	case <-probe.returned:
	case <-time.After(3 * time.Second):
		t.Fatal("OnChange entered but did not return within 3s: it is deadlocked reading the configuration while the watcher goroutine holds the config write lock across the call. The watcher is now stuck permanently and every later reload is silently lost.")
	}

	// The hook sees the reloaded values, not the ones it started with: the
	// write lock is released after c.vals has been replaced, so by the time
	// the hook reads, the new values are already installed.
	select {
	case got := <-probe.observed:
		if got == "" {
			t.Error(`the hook read an empty "key": the values were not readable from inside OnChange`)
		}
		if got == "v0" {
			t.Error(`the hook read the startup value "v0": OnChange ran against values the reload should have replaced`)
		}
	default:
		t.Error("the hook returned without recording a read")
	}
}

// The first load, for contrast. This one is green both with OnChange inside
// the write lock and with it outside, and that is the point worth pinning:
// startup does not go through the watcher's lock at all. NewConfig's Init
// scans the entity and never calls OnChange, and sdk/config.Setup runs the
// startup work itself (_cfg.Init()) on the caller's goroutine with no config
// lock held. A guard written on this path would therefore have caught nothing
// - the defect only ever showed up on reload, which is why "all the tests
// pass" and "the first configuration change in production hangs" were both
// true at the same time.
func TestFirstLoadNeitherCallsOnChangeNorHoldsALockAgainstReaders(t *testing.T) {
	src := memory.NewSource(memory.WithJSON([]byte(`{"key": "v0"}`)))

	probe := newReloadProbe()
	conf, err := NewConfig(WithSource(src), WithEntity(probe))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	t.Cleanup(func() { _ = conf.Close() })
	probe.attach(conf)

	select {
	case <-probe.entered:
		t.Fatal("the first load called OnChange; it is only meant to fire on reload, and callers rely on that to run startup work exactly once")
	case <-time.After(200 * time.Millisecond):
	}

	// Startup work as Setup does it: the hook body, on the caller's
	// goroutine, right after NewConfig returned.
	done := make(chan struct{})
	go func() {
		defer close(done)
		probe.OnChange()
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("reading the configuration on the startup path blocked for 3s: NewConfig left a lock held after it returned")
	}

	select {
	case got := <-probe.observed:
		if got != "v0" {
			t.Errorf(`startup read "key" = %q, want "v0"`, got)
		}
	default:
		t.Error("the startup read recorded nothing")
	}
}

// writeUntil pushes a distinct changeset at the source until the hook is
// entered or the budget runs out. Each write carries a different value so the
// loader's version and content checks cannot coalesce them away.
func writeUntil(t *testing.T, src source.Source, entered <-chan struct{}, budget time.Duration) bool {
	t.Helper()

	deadline := time.Now().Add(budget)
	for i := 1; ; i++ {
		cs := &source.ChangeSet{
			Data:   []byte(fmt.Sprintf(`{"key": "v%d"}`, i)),
			Format: "json",
		}
		if err := src.Write(cs); err != nil {
			t.Fatalf("write changeset: %v", err)
		}

		select {
		case <-entered:
			return true
		case <-time.After(50 * time.Millisecond):
		}

		if time.Now().After(deadline) {
			return false
		}
	}
}
