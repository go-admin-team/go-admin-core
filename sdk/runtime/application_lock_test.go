package runtime

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// sync.RWMutex is not reentrant: taking the read lock while already holding
// the write lock deadlocks the goroutine. Every accessor below runs inside a
// locked section, so none of them may call GetDefaultTenant.
//
// These calls used to hang forever. They were never exercised by any consumer,
// which is why the defect survived unnoticed — hence this guard.

func mustReturn(t *testing.T, name string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Errorf("%s did not return within 2s: lock reentrancy deadlock", name)
	}
}

func TestSingleTenantAccessorsDoNotDeadlock(t *testing.T) {
	cases := []struct {
		name string
		fn   func()
	}{
		{"SetDb", func() { NewConfig().SetDb(&gorm.DB{}) }},
		{"SetApp", func() { NewConfig().SetApp("app") }},
		{"SetCasbin", func() { NewConfig().SetCasbin(nil) }},
		{"SetCasbinExclude", func() { NewConfig().SetCasbinExclude(nil) }},
		{"SetCrontab", func() { NewConfig().SetCrontab(nil) }},
		{"SetHandler", func() { NewConfig().SetHandler(nil) }},
		{"GetHandler", func() { NewConfig().GetHandler() }},
		{"GetConfig", func() { NewConfig().GetConfig() }},
		{"GetDb", func() { NewConfig().GetDb() }},
		{"GetCasbin", func() { NewConfig().GetCasbin() }},
		{"GetCrontab", func() { NewConfig().GetCrontab() }},
		{"GetConfigValue", func() { NewConfig().GetConfigValue("k") }},
		{"SetConfigValue", func() { NewConfig().SetConfigValue("k", "v") }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mustReturn(t, c.name, c.fn)
		})
	}
}

// The single-tenant setters must write under the default tenant key, i.e. be
// equivalent to their explicit *ByTenant counterparts.
func TestSingleTenantSettersUseDefaultTenant(t *testing.T) {
	app := NewConfig()
	db := &gorm.DB{}
	app.SetDb(db)

	if got := app.GetDbByTenant(DefaultTenant); got != db {
		t.Errorf("SetDb did not store under %q", DefaultTenant)
	}

	app.SetConfigValue("key", "value")
	if got := app.GetConfigValueByTenant(DefaultTenant, "key"); got != "value" {
		t.Errorf("SetConfigValue did not store under %q, got %v", DefaultTenant, got)
	}
}

// Acceptance 14: the same guard applied to everything this batch put behind
// the mutex. The failure mode is identical - a method that takes the lock and
// then calls another one that takes it hangs forever - and it is invisible
// until the one call order that triggers it happens in production.
func TestRegistryAccessorsDoNotDeadlock(t *testing.T) {
	gin.SetMode(gin.TestMode)
	noop := func() {}

	cases := []struct {
		name string
		fn   func()
	}{
		{"SetBefore", func() { NewConfig().SetBefore(noop) }},
		{"SetBeforeWith", func() { NewConfig().SetBeforeWith(noop, WithName("x"), WithFatal()) }},
		{"GetBefore", func() { NewConfig().GetBefore() }},
		{"RunBefore", func() { NewConfig().RunBefore() }},
		{"BeforeSealed", func() { NewConfig().BeforeSealed() }},
		{"SetAppRouters", func() { NewConfig().SetAppRouters(noop) }},
		{"SetAppRoutersWith", func() { NewConfig().SetAppRoutersWith(noop, WithName("x")) }},
		{"GetAppRouters", func() { NewConfig().GetAppRouters() }},
		{"RunAppRouters", func() { NewConfig().RunAppRouters() }},
		{"AppRoutersSealed", func() { NewConfig().AppRoutersSealed() }},
		{"SetEngine", func() { NewConfig().SetEngine(gin.New()) }},
		{"GetEngine", func() { NewConfig().GetEngine() }},
		{"GetRouter", func() { NewConfig().GetRouter() }},
		{"GetRouter with a gin engine", func() {
			app := NewConfig()
			app.SetEngine(gin.New())
			app.GetRouter()
		}},
		// The one that is not hypothetical: app/demo/router/router.go reads and
		// writes the engine from inside the very callback RunAppRouters
		// executes. Running callbacks under the lock hangs right here.
		{"RunAppRouters with a callback that takes the lock", func() {
			app := NewConfig()
			app.SetAppRouters(func() {
				if app.GetEngine() == nil {
					app.SetEngine(gin.New())
				}
			})
			app.RunAppRouters()
		}},
		{"RunBefore with a callback that takes the lock", func() {
			app := NewConfig()
			app.SetBefore(func() { app.SetConfigValue("k", "v") })
			app.RunBefore()
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mustReturn(t, c.name, c.fn)
		})
	}
}
