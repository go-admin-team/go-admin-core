package database

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

type DBConfig struct {
	dsn             string
	connMaxIdleTime int
	connMaxLifetime int
	maxIdleConns    int
	maxOpenConns    int
	registers       []ResolverConfigure
}

// NewConfigure 初始化 Configure
func NewConfigure(
	dsn string,
	maxIdleConns,
	maxOpenConns,
	connMaxIdleTime,
	connMaxLifetime int,
	registers []ResolverConfigure) Configure {
	return &DBConfig{
		dsn:             dsn,
		connMaxIdleTime: connMaxIdleTime,
		connMaxLifetime: connMaxLifetime,
		maxIdleConns:    maxIdleConns,
		maxOpenConns:    maxOpenConns,
		registers:       registers,
	}
}

// Init 获取db，⚠️注意：读写分离只能配置一组
func (e *DBConfig) Init(config *gorm.Config, open func(string) gorm.Dialector) (*gorm.DB, error) {
	db, err := gorm.Open(open(e.dsn), config)
	if err != nil {
		return nil, err
	}
	var register *dbresolver.DBResolver
	for i := range e.registers {
		register = e.registers[i].Init(register, open)
	}
	if register == nil {
		register = dbresolver.Register(dbresolver.Config{})
	}
	if e.connMaxIdleTime > 0 {
		register = register.SetConnMaxIdleTime(time.Duration(e.connMaxIdleTime) * time.Second)
	}
	if e.connMaxLifetime > 0 {
		register = register.SetConnMaxLifetime(time.Duration(e.connMaxLifetime) * time.Second)
	}
	if e.maxOpenConns > 0 {
		register = register.SetMaxOpenConns(e.maxOpenConns)
	}
	if e.maxIdleConns > 0 {
		register = register.SetMaxIdleConns(e.maxIdleConns)
	}
	if register != nil {
		err = db.Use(register)
	}
	return db, err
}

type DBResolverConfig struct {
	sources  []string
	replicas []string
	policy   string
	tables   []interface{}
}

// NewResolverConfigure 初始化 ResolverConfigure
func NewResolverConfigure(sources, replicas []string, policy string, tables []string) ResolverConfigure {
	data := make([]interface{}, len(tables))
	for i := range tables {
		data[i] = tables[i]
	}
	return &DBResolverConfig{
		sources:  sources,
		replicas: replicas,
		policy:   policy,
		tables:   data,
	}
}

func (e *DBResolverConfig) Init(
	register *dbresolver.DBResolver,
	open func(string) gorm.Dialector) *dbresolver.DBResolver {
	if len(e.tables) == 0 && len(e.sources) == 0 && len(e.replicas) == 0 {
		return register
	}
	var config dbresolver.Config
	if len(e.sources) > 0 {
		config.Sources = make([]gorm.Dialector, len(e.sources))
		for i := range e.sources {
			config.Sources[i] = open(e.sources[i])
		}
	}
	if len(e.replicas) > 0 {
		config.Replicas = make([]gorm.Dialector, len(e.replicas))
		for i := range e.replicas {
			config.Replicas[i] = open(e.replicas[i])
		}
	}
	if e.policy != "" {
		policy, ok := policies[e.policy]
		if ok {
			config.Policy = policy
		}
	}
	if register == nil {
		register = dbresolver.Register(config, e.tables...)
		return register
	}
	register = register.Register(config, e.tables...)
	return register
}
