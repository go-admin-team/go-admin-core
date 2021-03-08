package runtime

import (
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/go-admin-team/go-admin-core/logger"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type Application struct {
	dbs     map[string]*gorm.DB
	casbins map[string]*casbin.SyncedEnforcer
	engine  http.Handler
	crontab map[string]*cron.Cron
}

// SetDb 设置对应key的db
func (c *Application) SetDb(key string, db *gorm.DB) {
	c.dbs[key] = db
}

// GetDb 获取所有map里的db数据
func (c *Application) GetDb() map[string]*gorm.DB {
	return c.dbs
}

// GetDbByKey 根据key获取db
func (c *Application) GetDbByKey(key string) *gorm.DB {
	if db, ok := c.dbs["*"]; ok {
		return db
	}
	return c.dbs[key]
}

func (c *Application) SetCasbin(key string, enforcer *casbin.SyncedEnforcer) {
	c.casbins[key] = enforcer
}

// GetCasbinKey 根据key获取casbin
func (c *Application) GetCasbinKey(key string) *casbin.SyncedEnforcer {
	if e, ok := c.casbins["*"]; ok {
		return e
	}
	return c.casbins[key]
}

// SetEngine 设置路由引擎
func (c *Application) SetEngine(engine http.Handler) {
	c.engine = engine
}

// GetEngine 获取路由引擎
func (c *Application) GetEngine() http.Handler {
	return c.engine
}

// SetLogger 设置日志组件
func (c *Application) SetLogger(l logger.Logger) {
	logger.DefaultLogger = l
}

// GetLogger 获取日志组件
func (c *Application) GetLogger() logger.Logger {
	return logger.DefaultLogger
}

// NewConfig 默认值
func NewConfig() *Application {
	return &Application{
		dbs:     make(map[string]*gorm.DB),
		casbins: make(map[string]*casbin.SyncedEnforcer),
		crontab: make(map[string]*cron.Cron),
	}
}

// SetCrontab 设置对应key的crontab
func (c *Application) SetCrontab(key string, crontab *cron.Cron) {
	c.crontab[key] = crontab
}

// GetCrontab 获取所有map里的crontab数据
func (c *Application) GetCrontab() map[string]*cron.Cron {
	return c.crontab
}

// GetCrontabKey 根据key获取crontab
func (c *Application) GetCrontabKey(key string) *cron.Cron {
	if e, ok := c.crontab["*"]; ok {
		return e
	}
	return c.crontab[key]
}
