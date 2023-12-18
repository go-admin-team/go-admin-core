package runtime

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/go-admin-team/go-admin-core/logger"
	"github.com/go-admin-team/go-admin-core/storage"
	"github.com/go-admin-team/go-admin-core/storage/queue"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type Application struct {
	dbs           map[string]*gorm.DB
	casbins       map[string]*casbin.SyncedEnforcer
	engine        http.Handler
	crontab       map[string]*cron.Cron
	mux           sync.RWMutex
	middlewares   map[string]interface{}
	cache         storage.AdapterCache
	queue         storage.AdapterQueue
	locker        storage.AdapterLocker
	memoryQueue   storage.AdapterQueue
	handler       map[string][]func(r *gin.RouterGroup, hand ...*gin.HandlerFunc)
	routers       []Router
	configs       map[string]map[string]interface{} // 系统参数
	appRouters    []func()                          // app路由
	casbinExclude map[string]interface{}            // casbin排除
	before        []func()                          // 启动前执行
	app           map[string]interface{}            // app
}

type Router struct {
	HttpMethod, RelativePath, Handler string
}

type Routers struct {
	List []Router
}

func (e *Application) SetBefore(f func()) {
	e.before = append(e.before, f)
}

func (e *Application) GetBefore() []func() {
	return e.before
}

// SetCasbinExclude 设置对应key的Exclude
func (e *Application) SetCasbinExclude(key string, list interface{}) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.casbinExclude[key] = list
}

// GetCasbinExclude 获取所有map里的Exclude数据
func (e *Application) GetCasbinExclude() map[string]interface{} {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.casbinExclude
}

// GetCasbinExcludeByKey 根据key获取Exclude
func (e *Application) GetCasbinExcludeByKey(key string) interface{} {
	e.mux.Lock()
	defer e.mux.Unlock()
	if exclude, ok := e.casbinExclude["*"]; ok {
		return exclude
	}
	return e.casbinExclude[key]
}

// SetApp 设置对应key的app
func (e *Application) SetApp(key string, app interface{}) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.app[key] = app
}

// GetApp 获取所有map里的app数据
func (e *Application) GetApp() map[string]interface{} {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.app
}

// GetAppByKey 根据key获取app
func (e *Application) GetAppByKey(key string) interface{} {
	e.mux.Lock()
	defer e.mux.Unlock()
	if app, ok := e.app["*"]; ok {
		return app
	}
	return e.app[key]
}

// SetDb 设置对应key的db
func (e *Application) SetDb(key string, db *gorm.DB) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.dbs[key] = db
}

// GetDb 获取所有map里的db数据
func (e *Application) GetDb() map[string]*gorm.DB {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.dbs
}

// GetDbByKey 根据key获取db
func (e *Application) GetDbByKey(key string) *gorm.DB {
	e.mux.Lock()
	defer e.mux.Unlock()
	if db, ok := e.dbs["*"]; ok {
		return db
	}
	return e.dbs[key]
}

func (e *Application) SetCasbin(key string, enforcer *casbin.SyncedEnforcer) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.casbins[key] = enforcer
}

func (e *Application) GetCasbin() map[string]*casbin.SyncedEnforcer {
	return e.casbins
}

// GetCasbinKey 根据key获取casbin
func (e *Application) GetCasbinKey(key string) *casbin.SyncedEnforcer {
	e.mux.Lock()
	defer e.mux.Unlock()
	if e, ok := e.casbins["*"]; ok {
		return e
	}
	return e.casbins[key]
}

// SetEngine 设置路由引擎
func (e *Application) SetEngine(engine http.Handler) {
	e.engine = engine
}

// GetEngine 获取路由引擎
func (e *Application) GetEngine() http.Handler {
	return e.engine
}

// GetRouter 获取路由表
func (e *Application) GetRouter() []Router {
	return e.setRouter()
}

// setRouter 设置路由表
func (e *Application) setRouter() []Router {
	switch e.engine.(type) {
	case *gin.Engine:
		routers := e.engine.(*gin.Engine).Routes()
		for _, router := range routers {
			e.routers = append(e.routers, Router{RelativePath: router.Path, Handler: router.Handler, HttpMethod: router.Method})
		}
	}
	return e.routers
}

// SetLogger 设置日志组件
func (e *Application) SetLogger(l logger.Logger) {
	logger.DefaultLogger = l
}

// GetLogger 获取日志组件
func (e *Application) GetLogger() logger.Logger {
	return logger.DefaultLogger
}

// NewConfig 默认值
func NewConfig() *Application {
	return &Application{
		dbs:           make(map[string]*gorm.DB),
		casbins:       make(map[string]*casbin.SyncedEnforcer),
		crontab:       make(map[string]*cron.Cron),
		middlewares:   make(map[string]interface{}),
		memoryQueue:   queue.NewMemory(10000),
		handler:       make(map[string][]func(r *gin.RouterGroup, hand ...*gin.HandlerFunc)),
		routers:       make([]Router, 0),
		configs:       make(map[string]map[string]interface{}),
		casbinExclude: make(map[string]interface{}),
	}
}

// SetCrontab 设置对应key的crontab
func (e *Application) SetCrontab(key string, crontab *cron.Cron) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.crontab[key] = crontab
}

// GetCrontab 获取所有map里的crontab数据
func (e *Application) GetCrontab() map[string]*cron.Cron {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.crontab
}

// GetCrontabKey 根据key获取crontab
func (e *Application) GetCrontabKey(key string) *cron.Cron {
	e.mux.Lock()
	defer e.mux.Unlock()
	if e, ok := e.crontab["*"]; ok {
		return e
	}
	return e.crontab[key]
}

// SetMiddleware 设置中间件
func (e *Application) SetMiddleware(key string, middleware interface{}) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.middlewares[key] = middleware
}

// GetMiddleware 获取所有中间件
func (e *Application) GetMiddleware() map[string]interface{} {
	return e.middlewares
}

// GetMiddlewareKey 获取对应key的中间件
func (e *Application) GetMiddlewareKey(key string) interface{} {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.middlewares[key]
}

// SetCacheAdapter 设置缓存
func (e *Application) SetCacheAdapter(c storage.AdapterCache) {
	e.cache = c
}

// GetCacheAdapter 获取缓存
func (e *Application) GetCacheAdapter() storage.AdapterCache {
	return NewCache("", e.cache, "")
}

// GetCachePrefix 获取带租户标记的cache
func (e *Application) GetCachePrefix(key string) storage.AdapterCache {
	return NewCache(key, e.cache, "")
}

// SetQueueAdapter 设置队列适配器
func (e *Application) SetQueueAdapter(c storage.AdapterQueue) {
	e.queue = c
}

// GetQueueAdapter 获取队列适配器
func (e *Application) GetQueueAdapter() storage.AdapterQueue {
	return NewQueue("", e.queue)
}

// GetQueuePrefix 获取带租户标记的queue
func (e *Application) GetQueuePrefix(key string) storage.AdapterQueue {
	return NewQueue(key, e.queue)
}

// SetLockerAdapter 设置分布式锁
func (e *Application) SetLockerAdapter(c storage.AdapterLocker) {
	e.locker = c
}

// GetLockerAdapter 获取分布式锁
func (e *Application) GetLockerAdapter() storage.AdapterLocker {
	return NewLocker("", e.locker)
}

func (e *Application) GetLockerPrefix(key string) storage.AdapterLocker {
	return NewLocker(key, e.locker)
}

func (e *Application) SetHandler(key string, routerGroup func(r *gin.RouterGroup, hand ...*gin.HandlerFunc)) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.handler[key] = append(e.handler[key], routerGroup)
}

func (e *Application) GetHandler() map[string][]func(r *gin.RouterGroup, hand ...*gin.HandlerFunc) {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.handler
}

func (e *Application) GetHandlerPrefix(key string) []func(r *gin.RouterGroup, hand ...*gin.HandlerFunc) {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.handler[key]
}

// GetStreamMessage 获取队列需要用的message
func (e *Application) GetStreamMessage(id, stream string, value map[string]interface{}) (storage.Messager, error) {
	message := &queue.Message{}
	message.SetID(id)
	message.SetStream(stream)
	message.SetValues(value)
	return message, nil
}

func (e *Application) GetMemoryQueue(prefix string) storage.AdapterQueue {
	return NewQueue(prefix, e.memoryQueue)
}

// SetConfigByTenant 设置对应租户的config
func (e *Application) SetConfigByTenant(tenant string, value map[string]interface{}) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.configs[tenant] = value
}

// SetConfig 设置对应key的config
func (e *Application) SetConfig(tenant, key string, value interface{}) {
	e.mux.Lock()
	defer e.mux.Unlock()
	if _, ok := e.configs[tenant]; !ok {
		e.configs[tenant] = make(map[string]interface{})
	}
	e.configs[tenant][key] = value
}

// GetConfig 获取对应key的config
func (e *Application) GetConfig(tenant, key string) interface{} {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.configs[tenant][key]
}

// GetConfigByTenant 获取对应租户的config
func (e *Application) GetConfigByTenant(tenant string) interface{} {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.configs[tenant]
}

// SetAppRouters 设置app的路由
func (e *Application) SetAppRouters(appRouters func()) {
	e.appRouters = append(e.appRouters, appRouters)
}

// GetAppRouters 获取app的路由
func (e *Application) GetAppRouters() []func() {
	return e.appRouters
}
