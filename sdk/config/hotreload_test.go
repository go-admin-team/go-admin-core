//lint:file-ignore SA1019 Exercises the AdapterCache/AdapterQueue setters that the reload path actually calls; the deprecation is carried by the storage package itself.

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	coreconfig "github.com/go-admin-team/go-admin-core/v2/config"
	"github.com/go-admin-team/go-admin-core/v2/config/source/file"
	"github.com/go-admin-team/go-admin-core/v2/sdk"
	"github.com/go-admin-team/go-admin-core/v2/sdk/runtime"
	"github.com/go-admin-team/go-admin-core/v2/storage"
	"gorm.io/gorm"
)

// labelled adapters carry the configuration value they were built from, so the
// assertion is "the resource was replaced", not "a setter did not error".
type labelledCache struct {
	storage.AdapterCache
	label string
}

func (c *labelledCache) String() string { return c.label }

type labelledQueue struct {
	storage.AdapterQueue
	label string
}

func (q *labelledQueue) String() string { return q.label }

const settingsTemplate = `settings:
  application:
    host: 0.0.0.0
    port: 8000
    name: %s
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

// Acceptance 11: the whole reload path, through the real file watcher.
//
// This is the risk the seal was most likely to break, and it would have broken
// it silently: the configuration file changes, core re-runs every setup
// callback while requests are in flight, and if the registries' seal had been
// applied to the resource fields the new database would simply not be
// installed. Nothing would log, nothing would fail - the old connection would
// just keep being used.
//
// Setup replaces the package-level _cfg, coreconfig.DefaultConfig and the
// global logger, so this test must not run in parallel with anything.
func TestConfigReloadReplacesResourcesAfterTheRegistriesAreSealed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.yml")
	writeSettings(t, path, "v1")

	// The registries are process-wide and the seal is sticky, so a test that
	// trips it has to hand back a clean one - otherwise every later test in
	// the binary quietly loses its registrations.
	previousRuntime := sdk.Runtime
	t.Cleanup(func() { sdk.Runtime = previousRuntime })
	sdk.Runtime = runtime.NewConfig()

	previousCfg := _cfg
	previousDefault := coreconfig.DefaultConfig
	t.Cleanup(func() {
		if coreconfig.DefaultConfig != nil && coreconfig.DefaultConfig != previousDefault {
			_ = coreconfig.DefaultConfig.Close()
		}
		_cfg = previousCfg
		coreconfig.DefaultConfig = previousDefault
		// logger.DefaultLogger is deliberately left as Setup left it. The
		// watcher goroutine outlives Close by a moment and reads that global
		// on its way out, so restoring it here would be a race this test
		// creates rather than one it finds.
	})

	// Startup order as a server has it: the registries run and close first,
	// then the resources are installed by the setup callbacks.
	sdk.Runtime.RunBefore()
	sdk.Runtime.RunAppRouters()
	if !sdk.Runtime.BeforeSealed() || !sdk.Runtime.AppRoutersSealed() {
		t.Fatal("the registries are not sealed; this test would prove nothing")
	}

	setup := func() {
		label := ApplicationConfig.Name
		sdk.Runtime.SetCacheAdapter(&labelledCache{label: label})
		sdk.Runtime.SetQueueAdapter(&labelledQueue{label: label})
		sdk.Runtime.SetDbByTenant(label, &gorm.DB{})
	}

	Setup(file.NewSource(file.WithPath(path)), setup)

	if got := sdk.Runtime.GetCacheAdapter().String(); got != "v1" {
		t.Fatalf("the first load installed %q, want v1", got)
	}

	writeSettings(t, path, "v2")

	// The watcher is a goroutine and fsnotify coalesces, so the only honest
	// way to wait is to poll for the effect.
	waitFor(t, 15*time.Second, func() bool {
		return sdk.Runtime.GetCacheAdapter().String() == "v2"
	}, "the cache adapter was never replaced after the configuration changed")

	if got := sdk.Runtime.GetQueueAdapter().String(); got != "v2" {
		t.Errorf("the queue adapter is %q, want v2", got)
	}
	if sdk.Runtime.GetDbByTenant("v2") == nil {
		t.Error("the reload did not install a database for the new tenant")
	}
	if !sdk.Runtime.BeforeSealed() || !sdk.Runtime.AppRoutersSealed() {
		t.Error("the reload reopened a sealed registry")
	}
}

func writeSettings(t *testing.T, path, name string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(fmt.Sprintf(settingsTemplate, name)), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func waitFor(t *testing.T, limit time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal(msg)
}
