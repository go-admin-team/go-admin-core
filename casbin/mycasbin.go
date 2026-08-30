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

// ReloadInterval is how often an enforcer reloads its policy from the
// database.
//
// A policy written on one instance reaches the others no other way: the
// adapter writes to the database, and every other process keeps serving what
// it loaded at startup. Set this to zero before the first Setup to opt out —
// a single-instance deployment gains nothing from the query.
var ReloadInterval = time.Minute

var (
	enforcersMu sync.Mutex
	enforcers   = map[string]*casbin.SyncedEnforcer{}
)

// Setup returns the enforcer for key, building it from db on first use.
//
// key names the policy source. An enforcer serves the policy it loaded, so a
// deployment that reads from more than one database - the multi-tenant
// configuration, where each host has its own - has to pass a distinct key per
// database. Passing one key for all of them hands every caller the enforcer
// built from whichever database arrived first, and the rest are then
// authorized against a policy table that is not theirs. "" is the right key
// for a single database.
func Setup(db *gorm.DB, key string) *casbin.SyncedEnforcer {
	enforcersMu.Lock()
	defer enforcersMu.Unlock()

	if e, ok := enforcers[key]; ok {
		return e
	}

	Apter, err := gormAdapter.NewAdapterByDBUseTableName(db, "", "casbin_rule")
	if err != nil && err.Error() != "invalid DDL" {
		panic(err)
	}

	m, err := model.NewModelFromString(text)
	if err != nil {
		panic(err)
	}
	e, err := casbin.NewSyncedEnforcer(m, Apter)
	if err != nil {
		panic(err)
	}

	if err := e.LoadPolicy(); err != nil {
		panic(err)
	}

	// Without this the policy is whatever it was at startup. UpdateCallback
	// below is written for a watcher, which would make this immediate rather
	// than periodic; polling needs no second piece of infrastructure and
	// closes the gap today.
	if ReloadInterval > 0 {
		e.StartAutoLoadPolicy(ReloadInterval)
	}

	// Casbin v3 enables logging by default; EnableLog is not needed.

	enforcers[key] = e
	return e
}

// UpdateCallback reloads the policy, for use as a watcher's update callback.
//
// Setup polls instead, which needs no broker. A deployment that already has
// one can register a watcher against this and have the change arrive at once
// rather than within ReloadInterval:
//
//	enforcer.SetWatcher(w)
//	w.SetUpdateCallback(mycasbin.UpdateCallback)
//
// The message carries no tenant, so every enforcer reloads. That is a wasted
// query for the tenants whose policy did not change, and the alternative -
// reloading whichever one happens to be first - is wrong rather than slow.
func UpdateCallback(msg string) {
	l := logger.NewHelper(sdk.Runtime.GetLogger())
	l.Infof("casbin updateCallback msg: %v", msg)

	// Copied out so LoadPolicy, which queries the database, does not hold the
	// lock that Setup needs.
	enforcersMu.Lock()
	built := make([]*casbin.SyncedEnforcer, 0, len(enforcers))
	for _, e := range enforcers {
		built = append(built, e)
	}
	enforcersMu.Unlock()

	for _, e := range built {
		if err := e.LoadPolicy(); err != nil {
			l.Errorf("casbin LoadPolicy err: %v", err)
		}
	}
}
