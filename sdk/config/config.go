package config

import (
	"fmt"
	"log"
	"log/slog"
	"sort"

	"github.com/go-admin-team/go-admin-core/v2/config"
	"github.com/go-admin-team/go-admin-core/v2/config/source"
)

var (
	ExtendConfig interface{}
	_cfg         *Settings
)

// Settings 兼容原先的配置结构
type Settings struct {
	Settings  Config `yaml:"settings"`
	callbacks []func()
}

func (e *Settings) runCallback() {
	for i := range e.callbacks {
		e.callbacks[i]()
	}
}

func (e *Settings) OnChange() {
	e.init()
	log.Println("config change and reload")
}

func (e *Settings) Init() {
	e.init()
	log.Println("config init")
}

func (e *Settings) init() {
	e.Settings.Logger.Setup()
	e.Settings.multiDatabase()
	e.runCallback()
}

// Config 配置集合
type Config struct {
	Application *Application          `yaml:"application"`
	Ssl         *Ssl                  `yaml:"ssl"`
	Logger      *Logger               `yaml:"logger"`
	Jwt         *Jwt                  `yaml:"jwt"`
	Database    *Database             `yaml:"database"`
	Databases   *map[string]*Database `yaml:"databases"`
	Gen         *Gen                  `yaml:"gen"`
	Cache       *Cache                `yaml:"cache"`
	Queue       *Queue                `yaml:"queue"`
	Extend      interface{}           `yaml:"extend"`
}

// multiDatabase fills in the databases map for a single-database setup, where
// the wildcard entry means "one database serves every tenant".
func (e *Config) multiDatabase() {
	if len(*e.Databases) == 0 {
		*e.Databases = map[string]*Database{
			"*": e.Database,
		}
		return
	}

	// The wildcard short-circuits the per-tenant lookup, so anything listed
	// beside it is dead configuration. Said out loud, because the symptom is
	// tenants quietly sharing one database rather than an error.
	if _, wildcard := (*e.Databases)["*"]; wildcard && len(*e.Databases) > 1 {
		others := make([]string, 0, len(*e.Databases)-1)
		for name := range *e.Databases {
			if name != "*" {
				others = append(others, name)
			}
		}
		sort.Strings(others)
		slog.Warn("config: the wildcard database entry hides the per-tenant ones, which will never be used",
			"ignored", others)
	}
}

// Setup 载入配置文件
func Setup(s source.Source,
	fs ...func()) {
	_cfg = &Settings{
		Settings: Config{
			Application: ApplicationConfig,
			Ssl:         SslConfig,
			Logger:      LoggerConfig,
			Jwt:         JwtConfig,
			Database:    DatabaseConfig,
			Databases:   &DatabasesConfig,
			Gen:         GenConfig,
			Cache:       CacheConfig,
			Queue:       QueueConfig,
			Extend:      ExtendConfig,
		},
		callbacks: fs,
	}
	var err error
	config.DefaultConfig, err = config.NewConfig(
		config.WithSource(s),
		config.WithEntity(_cfg),
	)
	handleError(err, "New config object fail")
	_cfg.Init()
}

func handleError(err error, msg string) {
	if err != nil {
		log.Fatal(fmt.Sprintf("%s: %s", msg, err.Error()))
	}
}

// GetConfig 获取配置对象
// 返回当前加载的框架配置，如果未初始化则返回 nil
func GetConfig() *Config {
	if _cfg == nil {
		return nil
	}
	return &_cfg.Settings
}
