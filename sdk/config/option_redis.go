package config

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
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
// Nothing evicts from it, because the client is handed to interfaces that have
// no shutdown of their own and a caller may still hold it. A configuration
// reload runs Setup again, so changing credentials adds an entry and leaves the
// previous client open for the life of the process. That is bounded by how many
// distinct destinations have been configured, and closing the old one would
// break whoever still holds it.
var clients struct {
	sync.Mutex
	byKey map[clientKey]*goredis.Client
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

	key := e.clientKey(o)

	clients.Lock()
	defer clients.Unlock()
	if c, ok := clients.byKey[key]; ok {
		return c, nil
	}

	client := goredis.NewClient(o)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	if clients.byKey == nil {
		clients.byKey = make(map[clientKey]*goredis.Client)
	}
	clients.byKey[key] = client
	return client, nil
}

// clientKey identifies a connection by everything that decides where a command
// lands and who it is issued as.
//
// Credentials and transport belong in it, not just the address: sharing a
// client between two configurations that differ only in password would make
// the wrong one appear to work, hiding the misconfiguration rather than
// reporting it. Pool size is deliberately absent, so tuning it does not split
// one server into two pools.
type clientKey struct {
	// From the resolved options, so that a url and the equivalent fields name
	// the same connection.
	network, addr, username, password string
	db                                int

	// tls.Config is not comparable, so the parts that decide which peer is
	// trusted are taken from the configuration that produced it. Certificate
	// files are compared by path: they are read once at startup.
	tls      bool
	certFile string
	keyFile  string
	caFile   string

	// Set by ParseURL for rediss:// and by ?skip_verify.
	serverName         string
	insecureSkipVerify bool
}

func (e RedisOptions) clientKey(o *goredis.Options) clientKey {
	k := clientKey{
		network:  o.Network,
		addr:     o.Addr,
		username: o.Username,
		password: o.Password,
		db:       o.DB,
		tls:      o.TLSConfig != nil,
	}
	if o.TLSConfig != nil {
		k.serverName = o.TLSConfig.ServerName
		k.insecureSkipVerify = o.TLSConfig.InsecureSkipVerify
	}
	if e.Tls != nil {
		k.certFile, k.keyFile, k.caFile = e.Tls.Cert, e.Tls.Key, e.Tls.Ca
	}
	return k
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
