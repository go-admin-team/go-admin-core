package mycasbin

import (
	"sync"
	"time"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	gormAdapter "github.com/casbin/gorm-adapter/v3"
	"github.com/go-admin-team/go-admin-core/v2/logger"
	"github.com/go-admin-team/go-admin-core/v2/sdk"
	"gorm.io/gorm"
)

// Initialize the model from a string.
var text = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && (keyMatch2(r.obj, p.obj) || keyMatch(r.obj, p.obj)) && (r.act == p.act || p.act == "*")
`

// ReloadInterval is how often the shared enforcer reloads its policy from the
// database.
//
// A policy written on one instance reaches the others no other way: the
// adapter writes to the database, and every other process keeps serving what
// it loaded at startup. Set this to zero before the first Setup to opt out —
// a single-instance deployment gains nothing from the query.
var ReloadInterval = time.Minute

var (
	enforcer *casbin.SyncedEnforcer
	once     sync.Once
)

func Setup(db *gorm.DB, _ string) *casbin.SyncedEnforcer {
	once.Do(func() {
		Apter, err := gormAdapter.NewAdapterByDBUseTableName(db, "", "casbin_rule")
		if err != nil && err.Error() != "invalid DDL" {
			panic(err)
		}

		m, err := model.NewModelFromString(text)
		if err != nil {
			panic(err)
		}
		enforcer, err = casbin.NewSyncedEnforcer(m, Apter)
		if err != nil {
			panic(err)
		}
		err = enforcer.LoadPolicy()
		if err != nil {
			panic(err)
		}

		// Without this the policy is whatever it was at startup. UpdateCallback
		// below is written for a watcher, which would make this immediate
		// rather than periodic; polling needs no second piece of infrastructure
		// and closes the gap today.
		if ReloadInterval > 0 {
			enforcer.StartAutoLoadPolicy(ReloadInterval)
		}

		// Casbin v3: 日志默认已启用，无需 EnableLog()
	})

	return enforcer
}

// UpdateCallback reloads the policy, for use as a watcher's update callback.
//
// Setup polls instead, which needs no broker. A deployment that already has
// one can register a watcher against this and have the change arrive at once
// rather than within ReloadInterval:
//
//	enforcer.SetWatcher(w)
//	w.SetUpdateCallback(mycasbin.UpdateCallback)
func UpdateCallback(msg string) {
	l := logger.NewHelper(sdk.Runtime.GetLogger())
	l.Infof("casbin updateCallback msg: %v", msg)
	err := enforcer.LoadPolicy()
	if err != nil {
		l.Errorf("casbin LoadPolicy err: %v", err)
	}
}
