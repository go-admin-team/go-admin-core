package config

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// RedisOptions describes how to reach a Redis server.
//
// Set URL, which understands redis:// and rediss:// and is what a managed
// provider hands out, or fill in the individual fields. URL wins when both are
// present.
//
// The json names are what the configuration file is keyed on, and they match
// the ones this repository used before Redis support was removed, so an older
// settings file still applies.
type RedisOptions struct {
	URL        string `yaml:"url" json:"url"`
	Network    string `yaml:"network" json:"network"`
	Addr       string `yaml:"addr" json:"addr"`
	Username   string `yaml:"username" json:"username"`
	Password   string `yaml:"password" json:"password"`
	DB         int    `yaml:"db" json:"db"`
	PoolSize   int    `yaml:"pool_size" json:"pool_size"`
	MaxRetries int    `yaml:"max_retries" json:"max_retries"`
	Tls        *Tls   `yaml:"tls" json:"tls"`
}

// Tls points at the certificate files for an encrypted connection.
type Tls struct {
	Cert string `yaml:"cert" json:"cert"`
	Key  string `yaml:"key" json:"key"`
	Ca   string `yaml:"ca" json:"ca"`
}

// GetRedisOptions converts the configuration into client options.
func (e RedisOptions) GetRedisOptions() (*goredis.Options, error) {
	if e.URL != "" {
		return goredis.ParseURL(e.URL)
	}
	if e.Addr == "" {
		return nil, errors.New("config: redis needs either url or addr")
	}

	o := &goredis.Options{
		Network:    e.Network,
		Addr:       e.Addr,
		Username:   e.Username,
		Password:   e.Password,
		DB:         e.DB,
		PoolSize:   e.PoolSize,
		MaxRetries: e.MaxRetries,
	}

	var err error
	o.TLSConfig, err = e.Tls.config()
	if err != nil {
		return nil, err
	}
	return o, nil
}

// setupTimeout bounds the connection check, so a wrong address fails the boot
// quickly instead of hanging it.
const setupTimeout = 5 * time.Second

// clients memoises connections by destination. Cache and queue are configured
// and built separately but normally point at one server, and without this each
// would open its own pool, doubling the connection count a managed Redis sees
// and paying a second handshake before the process can serve anything.
//
// Nothing evicts from it. The interfaces these clients are handed to have no
// shutdown of their own, so they live as long as the process either way.
var clients struct {
	sync.Mutex
	byTarget map[string]*goredis.Client
}

// Client connects and verifies the server answers, so that a wrong address
// fails the boot rather than the first request that needs the cache.
//
// The returned client is shared with every other caller naming the same
// destination, and is not closed by this package.
func (e RedisOptions) Client(ctx context.Context) (*goredis.Client, error) {
	o, err := e.GetRedisOptions()
	if err != nil {
		return nil, err
	}

	// Everything that decides which keyspace a command lands in. Two configs
	// differing only in pool size share a client; differing in db do not.
	target := fmt.Sprintf("%s|%s|%s|%d", o.Network, o.Addr, o.Username, o.DB)

	clients.Lock()
	defer clients.Unlock()
	if c, ok := clients.byTarget[target]; ok {
		return c, nil
	}

	client := goredis.NewClient(o)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	if clients.byTarget == nil {
		clients.byTarget = make(map[string]*goredis.Client)
	}
	clients.byTarget[target] = client
	return client, nil
}

// connect dials with the boot timeout applied.
func (e RedisOptions) connect() (*goredis.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), setupTimeout)
	defer cancel()
	return e.Client(ctx)
}

func (e *Tls) config() (*tls.Config, error) {
	if e == nil {
		return nil, nil
	}

	cert, err := tls.LoadX509KeyPair(e.Cert, e.Key)
	if err != nil {
		return nil, err
	}
	c := &tls.Config{Certificates: []tls.Certificate{cert}}

	if e.Ca == "" {
		return c, nil
	}
	ca, err := os.ReadFile(e.Ca)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, errors.New("config: no certificate found in " + e.Ca)
	}
	c.RootCAs = pool
	return c, nil
}
