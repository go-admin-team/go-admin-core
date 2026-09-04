package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	coreconfig "github.com/go-admin-team/go-admin-core/v2/config"
	"github.com/go-admin-team/go-admin-core/v2/config/source/file"
)

// resetExtend clears the process-wide registry for the duration of one test.
// extendTargets is deliberately process-wide in production (see extend.go),
// which means tests must not leak registrations into one another.
func resetExtend(t *testing.T) {
	t.Helper()
	extendMu.Lock()
	previous := extendTargets
	extendTargets = map[string]extendSlot{}
	extendMu.Unlock()
	t.Cleanup(func() {
		extendMu.Lock()
		extendTargets = previous
		extendMu.Unlock()
	})
}

// orderExtend and paymentExtend are deliberately the most ordinary shape a
// caller would reach for: plain structs, exported fields, no mutex, no
// json.Unmarshaler. Under the old target-mutating RegisterExtend this shape
// was unsafe to read while a reload was in flight; under the snapshot design
// it is the normal case, which TestRegisterExtendSurvivesConfigReload holds
// to a -race run to prove.
type orderExtend struct {
	Endpoint string
}

type paymentExtend struct {
	Endpoint string
	Timeout  int
}

// This is PRD acceptance 17: two callers each claim a section and neither
// sees, nor overwrites, the other's.
func TestRegisterExtendKeepsSectionsIndependent(t *testing.T) {
	resetExtend(t)

	order := RegisterExtend[orderExtend]("order")
	payment := RegisterExtend[paymentExtend]("payment")

	const blob = `{"extend":{
		"order":   {"Endpoint": "https://order.internal"},
		"payment": {"Endpoint": "https://payment.internal", "Timeout": 30}
	}}`
	decodeExtend(t, blob)

	if got := order().Endpoint; got != "https://order.internal" {
		t.Errorf("order().Endpoint = %q, want the order section's value", got)
	}
	if got := payment(); got.Endpoint != "https://payment.internal" || got.Timeout != 30 {
		t.Errorf("payment() = %+v, want the payment section's values", got)
	}
	// Neither section's fields leaked into the other's target.
	if order().Endpoint == payment().Endpoint {
		t.Fatal("order and payment ended up sharing a value; the sections were not kept apart")
	}
}

// The mandated counterproof: two callers registering the same key must fail
// loudly, not have the second one silently win. Without this, the failure
// mode is indistinguishable from a working system until the first caller's
// configuration mysteriously stops arriving.
func TestRegisterExtendPanicsOnDuplicateKey(t *testing.T) {
	resetExtend(t)

	RegisterExtend[orderExtend]("order")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("RegisterExtend must panic when a key is claimed twice, it returned normally instead")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "order") || !strings.Contains(msg, "twice") {
			t.Errorf("panic value %q does not identify the duplicate key or say it happened twice", msg)
		}
	}()
	// The second call's T does not even need to match the first's - the
	// registry is keyed by string, not by type, so this must panic exactly
	// as it would with a matching type.
	RegisterExtend[paymentExtend]("order")
}

func TestRegisterExtendPanicsOnEmptyKey(t *testing.T) {
	resetExtend(t)

	defer func() {
		if recover() == nil {
			t.Fatal("RegisterExtend must panic on an empty key")
		}
	}()
	RegisterExtend[orderExtend]("")
}

// There is no target argument any more for a caller to pass nil for - the
// old RegisterExtend(key, target) let a caller do that, and panicked on it;
// the new RegisterExtend[T](key) allocates its own T, so that failure mode
// cannot occur by construction. What must still hold is the guarantee that
// replaces it: the accessor is safe to call, and to dereference, from the
// moment RegisterExtend returns - a caller must never need a nil check to
// use it at startup, before any load has happened.
func TestRegisterExtendAccessorNeverReturnsNil(t *testing.T) {
	resetExtend(t)

	order := RegisterExtend[orderExtend]("order")
	if got := order(); got == nil {
		t.Fatal("the accessor returned nil before any load happened")
	}
}

// A registered key absent from the current configuration must be left alone,
// not zeroed or errored - the section simply was not in this particular
// config file (or this particular reload), which is a normal state, not a
// signal that the application making the claim did something wrong.
func TestExtendDispatcherIgnoresAnUnclaimedOrMissingSection(t *testing.T) {
	resetExtend(t)

	order := RegisterExtend[orderExtend]("order")
	decodeExtend(t, `{"extend":{"payment":{"Endpoint":"x"}}}`)

	if got := order().Endpoint; got != "" {
		t.Errorf("a section that was never present must not modify the target, got %+v", got)
	}
}

// Before RegisterExtend existed, the only mechanism was pointing the
// package-level ExtendConfig at a struct and letting the whole extend:
// section decode into it. RegisterExtend must not break that for a caller
// that has not adopted it - this is what "只加不减" means operationally.
func TestExtendDispatcherKeepsLegacyExtendConfigWorking(t *testing.T) {
	resetExtend(t)

	previous := ExtendConfig
	t.Cleanup(func() { ExtendConfig = previous })

	type legacy struct {
		AMap struct{ Key string }
	}
	var target legacy
	ExtendConfig = &target

	decodeExtend(t, `{"extend":{"AMap":{"Key":"abc"}}}`)

	if target.AMap.Key != "abc" {
		t.Errorf("legacy ExtendConfig was not populated, got %+v", target)
	}
}

// Both mechanisms are independent: a host that has not migrated off
// ExtendConfig still gets its data even while an application uses
// RegisterExtend for its own section.
func TestExtendDispatcherRunsLegacyAndRegisteredTargetsTogether(t *testing.T) {
	resetExtend(t)

	previous := ExtendConfig
	t.Cleanup(func() { ExtendConfig = previous })

	type legacy struct{ Host string }
	var hostCfg legacy
	ExtendConfig = &hostCfg

	order := RegisterExtend[orderExtend]("order")

	decodeExtend(t, `{"extend":{"Host":"h1","order":{"Endpoint":"e1"}}}`)

	if hostCfg.Host != "h1" {
		t.Errorf("legacy target = %+v, want Host h1", hostCfg)
	}
	if got := order().Endpoint; got != "e1" {
		t.Errorf("registered target endpoint = %q, want e1", got)
	}
}

// A malformed registered section must be reported against the key that
// caused it, not swallowed and not blamed on config as a whole.
func TestExtendDispatcherNamesTheFailingKey(t *testing.T) {
	resetExtend(t)

	RegisterExtend[orderExtend]("order")

	err := json.Unmarshal([]byte(`{"extend":{"order":["not","an","object"]}}`), &struct {
		Extend interface{} `json:"extend"`
	}{Extend: &extendDispatcher{}})
	if err == nil {
		t.Fatal("a section that does not match its target's shape must produce an error")
	}
	if !strings.Contains(err.Error(), "order") {
		t.Errorf("error %q does not name the offending key", err.Error())
	}
}

// decodeExtend runs the dispatcher exactly the way Setup wires it: as the
// Extend field of a struct being unmarshalled from JSON.
func decodeExtend(t *testing.T, blob string) {
	t.Helper()
	var w struct {
		Extend interface{} `json:"extend"`
	}
	w.Extend = &extendDispatcher{}
	if err := json.Unmarshal([]byte(blob), &w); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

const extendSettingsTemplate = `settings:
  application:
    host: 0.0.0.0
    port: 8000
    name: extend-test
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
  extend:
    order:
      Endpoint: %s
    payment:
      Endpoint: %s
`

// TestRegisterExtendSurvivesConfigReload is also this package's counterproof
// for the race the snapshot design exists to remove. order and payment are
// the plain, lock-free structs declared above - exactly the shape a caller
// reaches for without reading any concurrency notes first - registered
// through the real file loader and watcher (Setup installs one
// *extendDispatcher instance, and every reload calls Scan against the same
// entity). Two goroutines read them in a tight loop for the whole test while
// the watcher reloads the file underneath them; go test -race must come back
// clean, or the snapshot design has not actually removed the race
// RegisterExtend used to have.
//
// Mirrors TestConfigReloadReplacesResourcesAfterTheRegistriesAreSealed's
// save/restore of the package-level globals Setup touches.
func TestRegisterExtendSurvivesConfigReload(t *testing.T) {
	resetExtend(t)

	order := RegisterExtend[orderExtend]("order")
	payment := RegisterExtend[paymentExtend]("payment")

	dir := t.TempDir()
	path := filepath.Join(dir, "settings.yml")
	writeExtendSettings(t, path, "v1-order", "v1-payment")

	previousCfg := _cfg
	previousDefault := coreconfig.DefaultConfig
	t.Cleanup(func() {
		if coreconfig.DefaultConfig != nil && coreconfig.DefaultConfig != previousDefault {
			_ = coreconfig.DefaultConfig.Close()
		}
		_cfg = previousCfg
		coreconfig.DefaultConfig = previousDefault
	})

	// Read both sections in a tight loop, concurrently with every reload
	// below, for as long as the test runs - the counterproof itself. Neither
	// read touches a lock; what makes it safe is that order() and payment()
	// each return a pointer to a complete snapshot that apply() never
	// mutates in place once published.
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = order().Endpoint
			}
		}
	}()
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = payment().Endpoint
			}
		}
	}()
	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})

	Setup(file.NewSource(file.WithPath(path)))

	waitFor(t, 15*time.Second, func() bool {
		return order().Endpoint == "v1-order"
	}, "order's extend section was never populated by the initial load")
	waitFor(t, 15*time.Second, func() bool {
		return payment().Endpoint == "v1-payment"
	}, "payment's extend section was never populated by the initial load")

	writeExtendSettings(t, path, "v2-order", "v2-payment")

	waitFor(t, 15*time.Second, func() bool {
		return order().Endpoint == "v2-order"
	}, "order's extend section was never refilled after the configuration changed")
	waitFor(t, 15*time.Second, func() bool {
		return payment().Endpoint == "v2-payment"
	}, "payment's extend section was never refilled after the configuration changed")
}

func writeExtendSettings(t *testing.T, path, orderEndpoint, paymentEndpoint string) {
	t.Helper()
	body := fmt.Sprintf(extendSettingsTemplate, orderEndpoint, paymentEndpoint)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
