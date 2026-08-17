package config

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
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

	target := cacheKey(o)

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

// cacheKey identifies a connection by everything that decides where a command
// lands and who it is issued as.
//
// Credentials and transport belong in it, not just the address: sharing a
// client across two configs that differ only in password would make the wrong
// one appear to work, which hides the misconfiguration instead of reporting it.
// The result is hashed so no secret is held as a map key.
func cacheKey(o *goredis.Options) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%d\x00", o.Network, o.Addr, o.Username, o.Password, o.DB)

	if o.TLSConfig == nil {
		fmt.Fprint(h, "notls")
		return hex.EncodeToString(h.Sum(nil))
	}

	fmt.Fprintf(h, "tls\x00%s\x00%t\x00", o.TLSConfig.ServerName, o.TLSConfig.InsecureSkipVerify)
	// The certificates themselves, not how many there are: two configurations
	// presenting a different identity must not share a connection.
	for _, cert := range o.TLSConfig.Certificates {
		for _, der := range cert.Certificate {
			h.Write(der)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
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
