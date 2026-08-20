//lint:file-ignore SA1019 This file is part of the bridge to the deprecated
// AdapterCache. Referring to what it exists to keep working is the point of it;
// the deprecation is carried by storage.AdapterCache itself.

package runtime

import (
	"github.com/gin-gonic/gin"
	"net/http"

	"github.com/casbin/casbin/v3"
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

	SetBefore(f func())
	GetBefore() []func()

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

	// SetAppRouters set AppRouter
	SetAppRouters(appRouters func())
	GetAppRouters() []func()
}
