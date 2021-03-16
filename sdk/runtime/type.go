package runtime

import (
	"github.com/go-admin-team/go-admin-core/cache"
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/go-admin-team/go-admin-core/logger"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type Runtime interface {
	//多db设置，⚠️SetDbs不允许并发,可以根据自己的业务，例如app分库、host分库
	SetDb(key string, db *gorm.DB)
	GetDb() map[string]*gorm.DB
	GetDbByKey(key string) *gorm.DB

	SetCasbin(key string, enforcer *casbin.SyncedEnforcer)
	GetCasbin() map[string]*casbin.SyncedEnforcer
	GetCasbinKey(key string) *casbin.SyncedEnforcer

	//使用的路由
	SetEngine(engine http.Handler)
	GetEngine() http.Handler

	//使用go-admin定义的logger，参考来源go-micro
	SetLogger(logger logger.Logger)
	GetLogger() logger.Logger

	//crontab
	SetCrontab(key string, crontab *cron.Cron)
	GetCrontab() map[string]*cron.Cron
	GetCrontabKey(key string) *cron.Cron

	//middleware
	SetMiddleware(string, interface{})
	GetMiddleware() map[string]interface{}
	GetMiddlewareKey(key string) interface{}

	//cache
	SetCacheAdapter(cache.Adapter)
	GetCacheAdapter() cache.Adapter
}
