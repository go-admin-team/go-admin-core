//lint:file-ignore SA1019 Bridges to the deprecated AdapterCache; referring to what it exists to keep working is the point of the file, and the deprecation is carried by storage.AdapterCache itself.

package runtime

import (
	"github.com/gin-gonic/gin"
	"maps"
	"net/http"
	"slices"
	"sync"

	"github.com/casbin/casbin/v3"
	"github.com/go-admin-team/go-admin-core/v2/logger"
	"github.com/go-admin-team/go-admin-core/v2/storage"
	"github.com/go-admin-team/go-admin-core/v2/storage/queue"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// DefaultTenant 默认租户标识符
const DefaultTenant = "default"

type Config map[string]interface{}

type Application struct {
	dbs           map[string]*gorm.DB                                             // 数据库
	casbins       map[string]*casbin.SyncedEnforcer                               // casbin
	engine        http.Handler                                                    // 路由引擎
	crontab       map[string]*cron.Cron                                           // 定时任务
	mux           sync.RWMutex                                                    // 读写锁
	middlewares   map[string]interface{}                                          // 中间件
	cache         storage.AdapterCache                                            // 缓存
	queue         storage.AdapterQueue                                            // 队列
	memoryQueue   storage.AdapterQueue                                            // 内存队列
	handler       map[string][]func(r *gin.RouterGroup, hand ...*gin.HandlerFunc) // 路由处理器
	routers       []Router                                                        // 路由表
	configs       map[string]Config                                               // 系统参数
	appRouters    registry                                                        // app router registry
	casbinExclude map[string]interface{}                                          // casbin排除
	before        registry                                                        // startup callback registry
	defaultTenant string                                                          // 默认租户标识符
	app           map[string]interface{}                                          // app
}

// 通用的从map获取值的方法
func (e *Application) getFromMap(m map[string]interface{}, tenant string) interface{} {
	e.mux.RLock()
	defer e.mux.RUnlock()

	if val, ok := m["*"]; ok {
		return val
	}
	return m[tenant]
}

type Router struct {
	HttpMethod, RelativePath, Handler string
}

type Routers struct {
	List []Router
}

// SetDbByTenant 设置对应租户的db
func (e *Application) SetDbByTenant(tenant string, db *gorm.DB) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.dbs[tenant] = db
}

// SetDb 设置默认租户的db
func (e *Application) SetDb(db *gorm.DB) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.dbs[e.defaultTenantLocked()] = db
}

// GetDbByTenant 根据租户获取db
func (e *Application) GetDbByTenant(tenant string) *gorm.DB {
	e.mux.Lock()
	defer e.mux.Unlock()
	if db, ok := e.dbs["*"]; ok {
		return db
	}
	return e.dbs[tenant]
}

// GetDb 获取默认租户的数据库 (新增单租户快捷方法)
func (e *Application) GetDb() *gorm.DB {
	return e.GetDbByTenant(e.GetDefaultTenant())
}

// GetAllDb 获取所有租户的数据库 (原GetDb改名)
// GetAllDb returns a snapshot. Handing back the map itself was a race the
// lock could not fix: it was held across the return and released before the
// caller read anything.
func (e *Application) GetAllDb() map[string]*gorm.DB {
	e.mux.RLock()
	defer e.mux.RUnlock()
	return maps.Clone(e.dbs)
}

// SetAppByTenant 设置对应租户的app
func (e *Application) SetAppByTenant(key string, app interface{}) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.app[key] = app
}

// SetApp 设置默认租户的app
func (e *Application) SetApp(app interface{}) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.app[e.defaultTenantLocked()] = app
}

// GetApp 获取所有map里的app数据
func (e *Application) GetApp() map[string]interface{} {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.app
}

// GetAppByTenant 根据key获取app
func (e *Application) GetAppByTenant(tenant string) interface{} {
	return e.getFromMap(e.app, tenant)
}

// SetCasbinExcludeByTenant 设置对应租户的Exclude
func (e *Application) SetCasbinExcludeByTenant(tenant string, list interface{}) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.casbinExclude[tenant] = list
}

// SetCasbinExclude 设置默认租户的Exclude
func (e *Application) SetCasbinExclude(list interface{}) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.casbinExclude[e.defaultTenantLocked()] = list
}

// GetCasbinExclude 获取所有map里的Exclude数据
func (e *Application) GetCasbinExclude() map[string]interface{} {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.casbinExclude
}

// GetCasbinExcludeByTenant 根据租户获取Exclude
func (e *Application) GetCasbinExcludeByTenant(tenant string) interface{} {
	e.mux.Lock()
	defer e.mux.Unlock()
	if exclude, ok := e.casbinExclude["*"]; ok {
		return exclude
	}
	return e.casbinExclude[tenant]
}

func (e *Application) SetCasbinByTenant(tenant string, enforcer *casbin.SyncedEnforcer) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.casbins[tenant] = enforcer
}

// SetCasbin 设置默认租户的casbin
func (e *Application) SetCasbin(enforcer *casbin.SyncedEnforcer) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.casbins[e.defaultTenantLocked()] = enforcer
}

// GetAllCasbin 获取所有租户的casbin (原GetCasbin改名)
func (e *Application) GetAllCasbin() map[string]*casbin.SyncedEnforcer {
	e.mux.RLock()
	defer e.mux.RUnlock()
	return maps.Clone(e.casbins)
}

// GetCasbin 获取默认租户的casbin (新增单租户快捷方法)
func (e *Application) GetCasbin() *casbin.SyncedEnforcer {
	return e.GetCasbinByTenant(e.GetDefaultTenant())
}

// GetCasbinByTenant 根据租户获取casbin
func (e *Application) GetCasbinByTenant(tenant string) *casbin.SyncedEnforcer {
	e.mux.Lock()
	defer e.mux.Unlock()
	if e, ok := e.casbins["*"]; ok {
		return e
	}
	return e.casbins[tenant]
}

// SetEngine 设置路由引擎
//
// An app router callback is allowed to call this - the demo module does, on
// the path where no engine has been set yet - so the field is written after
// the server has started reading it elsewhere, and needs the lock.
func (e *Application) SetEngine(engine http.Handler) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.engine = engine
}

// GetEngine 获取路由引擎
func (e *Application) GetEngine() http.Handler {
	e.mux.RLock()
	defer e.mux.RUnlock()
	return e.engine
}

// GetRouter 获取路由表
func (e *Application) GetRouter() []Router {
	return e.setRouter()
}

// setRouter 设置路由表（并发安全）
func (e *Application) setRouter() []Router {
	// The engine is read once, under the lock, and used from the local copy.
	// The three bare reads this replaces raced with SetEngine, and this is the
	// one engine access that really does happen at run time: GetRouter is
	// reachable from the manual "sync API" endpoint, not just from startup.
	e.mux.RLock()
	engine := e.engine
	e.mux.RUnlock()

	ge, ok := engine.(*gin.Engine)
	if !ok {
		e.mux.RLock()
		defer e.mux.RUnlock()
		// Clone: returning the field itself lets the caller range over it once
		// the lock is gone, which is the race the lock looks like it prevents.
		return slices.Clone(e.routers)
	}

	// The table is rebuilt from the engine every time rather than appended to.
	// GetRouter is called more than once - the -a sync at startup, and the
	// manual sync endpoint at run time - and the original appended the full
	// route set to e.routers on each call, so the table grew by a multiple
	// every time. FirstOrCreate downstream kept that correct, but the cost of
	// a sync grew linearly with the number of syncs already done.
	//
	// Routes() and the loop run with no lock held: gin's route table is
	// read-only once registration is over, and holding e.mux across them would
	// block the config read path for as long as it takes. Only the assignment
	// takes the write lock, and the local snapshot is what is returned - the
	// caller can range over it after another goroutine has replaced the field,
	// which is the race that returning e.routers used to create.
	routers := ge.Routes()
	list := make([]Router, 0, len(routers))
	for _, router := range routers {
		list = append(list, Router{RelativePath: router.Path, Handler: router.Handler, HttpMethod: router.Method})
	}
	e.mux.Lock()
	e.routers = list
	e.mux.Unlock()
	return list
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
		configs:       make(map[string]Config),
		casbinExclude: make(map[string]interface{}),
		defaultTenant: DefaultTenant,
		app:           make(map[string]interface{}), // 添加初始化
	}
}

// SetDefaultTenant 设置默认租户
func (e *Application) SetDefaultTenant(tenant string) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.defaultTenant = tenant
}

// GetDefaultTenant 获取默认租户
func (e *Application) GetDefaultTenant() string {
	e.mux.RLock() // 使用读锁
	defer e.mux.RUnlock()
	return e.defaultTenant
}

// defaultTenantLocked returns the default tenant without taking e.mux.
// Callers must already hold the lock; sync.RWMutex is not reentrant, so
// calling GetDefaultTenant from a locked context deadlocks.
func (e *Application) defaultTenantLocked() string {
	return e.defaultTenant
}

// SetCrontabByTenant 设置对应租户的crontab
func (e *Application) SetCrontabByTenant(key string, crontab *cron.Cron) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.crontab[key] = crontab
}

// SetCrontab 设置默认租户的crontab
func (e *Application) SetCrontab(crontab *cron.Cron) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.crontab[e.defaultTenantLocked()] = crontab
}

// GetAllCrontab 获取所有租户的定时任务 (原GetCrontab改名)
func (e *Application) GetAllCrontab() map[string]*cron.Cron {
	e.mux.RLock()
	defer e.mux.RUnlock()
	return maps.Clone(e.crontab)
}

// GetCrontab 获取默认租户的定时任务 (新增单租户快捷方法)
func (e *Application) GetCrontab() *cron.Cron {
	return e.GetCrontabByTenant(e.GetDefaultTenant())
}

// GetCrontabByTenant 根据租户获取crontab
func (e *Application) GetCrontabByTenant(tenant string) *cron.Cron {
	e.mux.Lock()
	defer e.mux.Unlock()
	if e, ok := e.crontab["*"]; ok {
		return e
	}
	return e.crontab[tenant]
}

// SetMiddleware 设置租户中间件
func (e *Application) SetMiddleware(key string, middleware interface{}) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.middlewares[key] = middleware
}

// GetAllMiddleware 获取所有中间件
func (e *Application) GetAllMiddleware() map[string]interface{} {
	e.mux.RLock()
	defer e.mux.RUnlock()
	return maps.Clone(e.middlewares)
}

// GetMiddleware 获取对应的中间件
func (e *Application) GetMiddleware(key string) interface{} {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.middlewares[key]
}

// SetCacheAdapter 设置缓存
func (e *Application) SetCacheAdapter(c storage.AdapterCache) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.cache = c
}

// GetCacheAdapter 获取缓存
func (e *Application) GetCacheAdapter() storage.AdapterCache {
	return e.GetCacheAdapterPrefix("")
}

// GetCacheAdapterPrefix 获取prefix标记的cache
func (e *Application) GetCacheAdapterPrefix(prefix string) storage.AdapterCache {
	e.mux.RLock()
	cache := e.cache
	e.mux.RUnlock()
	return NewCache(prefix, cache, "")
}

// SetQueueAdapter 设置队列适配器
//
// Reloading the configuration calls this again while requests are in flight,
// so the field is guarded.
func (e *Application) SetQueueAdapter(c storage.AdapterQueue) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.queue = c
}

// queueAdapter returns the configured queue, falling back to the process-wide
// memory queue.
//
// The fallback has to be that one shared instance rather than a fresh queue:
// a producer and a consumer that both asked for "the queue" would otherwise be
// handed different ones, and every message would be dropped in silence.
func (e *Application) queueAdapter() storage.AdapterQueue {
	e.mux.RLock()
	defer e.mux.RUnlock()
	if e.queue != nil {
		return e.queue
	}
	return e.memoryQueue
}

// GetQueueAdapter 获取队列适配器
func (e *Application) GetQueueAdapter() storage.AdapterQueue {
	return NewQueue(e.GetDefaultTenant(), e.queueAdapter())
}

// GetQueuePrefix 获取标记的queue
//
// This is the queue the configuration selected. Prefer it over GetMemoryQueue,
// which ignores configuration and never leaves the process.
func (e *Application) GetQueuePrefix(key string) storage.AdapterQueue {
	return NewQueue(key, e.queueAdapter())
}

// GetQueue 获取默认租户的队列
func (e *Application) GetQueue() storage.AdapterQueue {
	return e.GetQueuePrefix(e.GetDefaultTenant())
}

func (e *Application) SetHandler(routerGroup func(r *gin.RouterGroup, hand ...*gin.HandlerFunc)) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.handler[e.defaultTenantLocked()] = append(e.handler[e.defaultTenantLocked()], routerGroup)
}

func (e *Application) SetHandlerByTenant(tenant string, routerGroup func(r *gin.RouterGroup, hand ...*gin.HandlerFunc)) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.handler[tenant] = append(e.handler[tenant], routerGroup)
}

func (e *Application) GetAllHandler() map[string][]func(r *gin.RouterGroup, hand ...*gin.HandlerFunc) {
	e.mux.RLock()
	defer e.mux.RUnlock()

	// The values are slices, so cloning the map is not enough: the copies
	// would share their arrays with the originals, and a caller appending to
	// one writes the slot the next SetHandlerByTenant writes.
	out := make(map[string][]func(r *gin.RouterGroup, hand ...*gin.HandlerFunc), len(e.handler))
	for tenant, fns := range e.handler {
		out[tenant] = slices.Clone(fns)
	}
	return out
}

func (e *Application) GetHandler() []func(r *gin.RouterGroup, hand ...*gin.HandlerFunc) {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.handler[e.defaultTenantLocked()]
}

func (e *Application) GetHandlerByTenant(tenant string) []func(r *gin.RouterGroup, hand ...*gin.HandlerFunc) {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.handler[tenant]
}

// GetStreamMessage 获取队列需要用的message
func (e *Application) GetStreamMessage(id, stream string, value map[string]interface{}) (storage.Messager, error) {
	message := &queue.Message{}
	message.SetID(id)
	message.SetStream(stream)
	message.SetValues(value)
	return message, nil
}

// GetMemoryQueue returns the in-process queue.
//
// Deprecated: use GetQueuePrefix. This always returns the in-process queue,
// whatever the configuration selected, so messages published through it are
// invisible to every other instance.
func (e *Application) GetMemoryQueue(prefix string) storage.AdapterQueue {
	return NewQueue(prefix, e.memoryQueue)
}

// SetConfigByTenant 设置对应租户的config
func (e *Application) SetConfigByTenant(tenant string, value map[string]interface{}) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.configs[tenant] = value
}

// SetConfigValueByTenant 设置对应租户的key的config
func (e *Application) SetConfigValueByTenant(tenant, key string, value interface{}) {
	e.mux.Lock()
	defer e.mux.Unlock()
	if _, ok := e.configs[tenant]; !ok {
		e.configs[tenant] = make(map[string]interface{})
	}
	e.configs[tenant][key] = value
}

// GetConfigValueByTenant 获取对应租户的config
func (e *Application) GetConfigValueByTenant(tenant, key string) interface{} {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.configs[tenant][key]
}

// GetConfigByTenant 获取对应租户的config
func (e *Application) GetConfigByTenant(tenant string) map[string]interface{} {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.configs[tenant]
}

// GetConfig 获取默认租户的config
func (e *Application) GetConfig() map[string]interface{} {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.configs[e.defaultTenantLocked()]
}

//func (e *Application) GetConfigValue(key string) interface{} {
//	e.mux.Lock()
//	defer e.mux.Unlock()
//	return e.configs[e.GetDefaultTenant()][key]
//}

// GetConfigValue 获取默认租户的配置值
func (e *Application) GetConfigValue(key string) interface{} {
	return e.GetConfigValueByTenant(e.GetDefaultTenant(), key)
}

// SetConfigValue 设置默认租户的配置值
func (e *Application) SetConfigValue(key string, value interface{}) {
	e.SetConfigValueByTenant(e.GetDefaultTenant(), key, value)
}
