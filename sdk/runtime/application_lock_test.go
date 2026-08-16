package runtime

import (
	"testing"
	"time"

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
