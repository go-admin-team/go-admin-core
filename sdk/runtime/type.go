//lint:file-ignore SA1019 Bridges to the deprecated AdapterCache; referring to what it exists to keep working is the point of the file, and the deprecation is carried by storage.AdapterCache itself.

package runtime

import (
	"context"
	"net/http"

	"github.com/casbin/casbin/v3"
	"github.com/gin-gonic/gin"
	"github.com/go-admin-team/go-admin-core/v2/logger"
	"github.com/go-admin-team/go-admin-core/v2/storage"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type Runtime interface {
	// SetDbByTenant 设置租户数据库
	SetDbByTenant(tenant string, db *gorm.DB)
	// SetDb 设置默认租户数据库
	SetDb(db *gorm.DB)
	GetDbByTenant(tenant string) *gorm.DB
	GetDb() *gorm.DB
	GetAllDb() map[string]*gorm.DB

	// SetBefore registers a callback to run when RunBefore is called.
	SetBefore(f func())
	// SetBeforeWith registers a before callback and says how a panic in it is
	// treated. With no options it is identical to SetBefore.
	SetBeforeWith(f func(), opts ...CallbackOption)
	// GetBefore returns the registered before callbacks.
	//
	// Deprecated: use RunBefore.
	GetBefore() []func()
	// RunBefore executes the before callbacks that have not run yet, in
	// registration order, and closes the registry.
	RunBefore()
	// BeforeSealed reports whether RunBefore has run, i.e. whether a further
	// SetBefore would be dropped.
	BeforeSealed() bool

	SetAppByTenant(tenant string, app interface{})
	SetApp(app interface{})
	GetApp() map[string]interface{}
	GetAppByTenant(tenant string) interface{}

	SetCasbinExcludeByTenant(tenant string, list interface{})
	SetCasbinExclude(list interface{})
	GetCasbinExclude() map[string]interface{}
	GetCasbinExcludeByTenant(tenant string) interface{}

	SetCasbinByTenant(tenant string, enforcer *casbin.SyncedEnforcer)
	SetCasbin(enforcer *casbin.SyncedEnforcer)
	GetAllCasbin() map[string]*casbin.SyncedEnforcer
	GetCasbin() *casbin.SyncedEnforcer
	GetCasbinByTenant(tenant string) *casbin.SyncedEnforcer

	// SetEngine 使用的路由
	SetEngine(engine http.Handler)
	GetEngine() http.Handler

	GetRouter() []Router

	// SetLogger 使用go-admin定义的logger，参考来源go-micro
	SetLogger(logger logger.Logger)
	GetLogger() logger.Logger

	SetDefaultTenant(tenant string)
	GetDefaultTenant() string

	// SetCrontabByTenant crontab
	SetCrontabByTenant(tenant string, crontab *cron.Cron)
	SetCrontab(crontab *cron.Cron)
	GetCrontab() *cron.Cron
	GetAllCrontab() map[string]*cron.Cron
	GetCrontabByTenant(tenant string) *cron.Cron

	// SetMiddleware middleware
	SetMiddleware(string, interface{})
	GetAllMiddleware() map[string]interface{}
	GetMiddleware(string) interface{}
	// GetHandlerFunc is GetMiddleware plus the gin.HandlerFunc assertion every
	// caller otherwise repeats, and false instead of a panic when the key is
	// unregistered or holds something else. See the well-known keys
	// JwtTokenCheck, RoleCheck and PermissionCheck.
	GetHandlerFunc(key string) (gin.HandlerFunc, bool)

	// SetCacheAdapter cache
	SetCacheAdapter(storage.AdapterCache)
	GetCacheAdapter() storage.AdapterCache
	GetCacheAdapterPrefix(string) storage.AdapterCache

	GetMemoryQueue(string) storage.AdapterQueue
	SetQueueAdapter(storage.AdapterQueue)
	GetQueueAdapter() storage.AdapterQueue
	GetQueuePrefix(string) storage.AdapterQueue

	SetHandler(routerGroup func(r *gin.RouterGroup, hand ...*gin.HandlerFunc))
	SetHandlerByTenant(tenant string, routerGroup func(r *gin.RouterGroup, hand ...*gin.HandlerFunc))
	GetAllHandler() map[string][]func(r *gin.RouterGroup, hand ...*gin.HandlerFunc)
	GetHandler() []func(r *gin.RouterGroup, hand ...*gin.HandlerFunc)
	GetHandlerByTenant(tenant string) []func(r *gin.RouterGroup, hand ...*gin.HandlerFunc)

	GetStreamMessage(id, stream string, value map[string]interface{}) (storage.Messager, error)

	// SetConfigByTenant 设置对应租户的config
	SetConfigByTenant(tenant string, value map[string]interface{})
	// SetConfigValueByTenant 设置对应租户的key对应的value
	SetConfigValueByTenant(tenant, key string, value interface{})
	SetConfigValue(key string, value interface{})
	GetConfigValueByTenant(tenant, key string) interface{}
	GetConfigByTenant(tenant string) map[string]interface{}
	GetConfig() map[string]interface{}
	GetConfigValue(key string) interface{}

	// SetAppRouters registers a router initialiser to run when RunAppRouters
	// is called.
	SetAppRouters(appRouters func())
	// SetAppRoutersWith registers a router initialiser and says how a panic in
	// it is treated. With no options it is identical to SetAppRouters.
	SetAppRoutersWith(appRouters func(), opts ...CallbackOption)
	// GetAppRouters returns the registered router initialisers.
	//
	// Deprecated: use RunAppRouters.
	GetAppRouters() []func()
	// RunAppRouters executes the router initialisers that have not run yet, in
	// registration order, and closes the registry.
	RunAppRouters()
	// AppRoutersSealed reports whether RunAppRouters has run, i.e. whether a
	// further SetAppRouters would be dropped.
	AppRoutersSealed() bool

	// SetPhase registers a callback to run when the application reaches the
	// given life-cycle phase.
	SetPhase(p Phase, f func(), opts ...CallbackOption)
	// RunPhase executes the callbacks registered for a phase that have not
	// run yet, in registration order.
	RunPhase(p Phase)
	// PhaseSealed reports whether a phase has already run, i.e. whether a
	// further SetPhase for it would be dropped.
	PhaseSealed(p Phase) bool
	// SetShutdown registers a cleanup callback for the BeforeExit phase,
	// taking the context that carries the shutdown budget.
	SetShutdown(f func(context.Context), opts ...CallbackOption)
	// RunShutdown runs the BeforeExit callbacks in reverse registration
	// order, returning ctx.Err() if the context ends first.
	RunShutdown(ctx context.Context) error
}
