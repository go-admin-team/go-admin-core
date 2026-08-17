package config

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"os"
	"testing"

	goredis "github.com/redis/go-redis/v9"
)

// The default must stay in memory. A deployment that never mentions Redis has
// to keep working with nothing running beside it.
func TestSetupDefaultsToMemory(t *testing.T) {
	c, err := Cache{}.Setup()
	if err != nil {
		t.Fatalf("Cache.Setup: %v", err)
	}
	if c.String() != "memory" {
		t.Errorf("an unconfigured cache must stay in memory, got %q", c.String())
	}

	q, err := Queue{}.Setup()
	if err != nil {
		t.Fatalf("Queue.Setup: %v", err)
	}
	if q.String() != "memory" {
		t.Errorf("an unconfigured queue must stay in memory, got %q", q.String())
	}
}

// Configuration reaches these structs as JSON, so the guarantee above has to
// survive decoding a settings file that has no cache or queue section.
func TestSettingsWithoutABackendSectionStayInMemory(t *testing.T) {
	const settings = `{"application":{"mode":"dev"}}`

	var decoded struct {
		Cache Cache `json:"cache"`
		Queue Queue `json:"queue"`
	}
	if err := json.Unmarshal([]byte(settings), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}

	c, err := decoded.Cache.Setup()
	if err != nil {
		t.Fatalf("Cache.Setup: %v", err)
	}
	if c.String() != "memory" {
		t.Errorf("a settings file with no cache section must stay in memory, got %q", c.String())
	}

	q, err := decoded.Queue.Setup()
	if err != nil {
		t.Fatalf("Queue.Setup: %v", err)
	}
	if q.String() != "memory" {
		t.Errorf("a settings file with no queue section must stay in memory, got %q", q.String())
	}
	if !decoded.Queue.Empty() {
		t.Error("a queue with no backend configured must report Empty")
	}
}

// A memory section is still honoured, and does not accidentally select Redis.
func TestMemorySectionSelectsMemory(t *testing.T) {
	q, err := Queue{Memory: &QueueMemory{PoolSize: 10}}.Setup()
	if err != nil {
		t.Fatalf("Queue.Setup: %v", err)
	}
	if q.String() != "memory" {
		t.Errorf("got %q, want memory", q.String())
	}
	if (Queue{Memory: &QueueMemory{}}).Empty() {
		t.Error("a configured memory queue must not report Empty")
	}
}

// A redis section that cannot be reached must fail the boot rather than fall
// back, otherwise an operator who asked for Redis silently gets a cache that is
// not shared between instances.
func TestUnreachableRedisIsAnError(t *testing.T) {
	// Port 1 is reserved and never listening.
	opts := RedisOptions{Addr: "127.0.0.1:1"}

	if _, err := (Cache{Redis: &opts}).Setup(); err == nil {
		t.Error("Cache.Setup must not fall back to memory when Redis is configured but unreachable")
	}
	if _, err := (Queue{Redis: &RedisQueue{RedisOptions: opts}}).Setup(); err == nil {
		t.Error("Queue.Setup must not fall back to memory when Redis is configured but unreachable")
	}
}

func TestRedisOptionsNeedsAnAddress(t *testing.T) {
	if _, err := (RedisOptions{}).GetRedisOptions(); err == nil {
		t.Error("a redis section with neither url nor addr must be rejected")
	}
}

func TestURLWinsOverFields(t *testing.T) {
	o, err := RedisOptions{
		URL:  "redis://localhost:6380/3",
		Addr: "127.0.0.1:9999",
	}.GetRedisOptions()
	if err != nil {
		t.Fatalf("GetRedisOptions: %v", err)
	}
	if o.Addr != "localhost:6380" || o.DB != 3 {
		t.Errorf("url must win, got addr %q db %d", o.Addr, o.DB)
	}
}

// With a reachable server, a redis section selects Redis for both.
func TestRedisSectionSelectsRedis(t *testing.T) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL is not set; skipping the Redis selection test")
	}

	// The bridge reports the backend's own identity, so this can assert what
	// was selected rather than only what was not.
	c, err := (Cache{Redis: &RedisOptions{URL: url}}).Setup()
	if err != nil {
		t.Fatalf("Cache.Setup: %v", err)
	}
	if c.String() != "redis" {
		t.Errorf("cache backend: got %q, want redis", c.String())
	}

	q, err := (Queue{Redis: &RedisQueue{RedisOptions: RedisOptions{URL: url}}}).Setup()
	if err != nil {
		t.Fatalf("Queue.Setup: %v", err)
	}
	defer q.Shutdown()
	if q.String() != "redis" {
		t.Errorf("queue backend: got %q, want redis", q.String())
	}
}

// Cache and queue are configured separately but normally name one server, and
// opening a pool for each doubles the connections a managed Redis sees.
func TestOneServerYieldsOneClient(t *testing.T) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL is not set; skipping the shared client test")
	}

	first, err := (RedisOptions{URL: url}).Client(context.Background())
	if err != nil {
		t.Fatalf("first connect: %v", err)
	}
	second, err := (RedisOptions{URL: url}).Client(context.Background())
	if err != nil {
		t.Fatalf("second connect: %v", err)
	}
	if first != second {
		t.Error("two configs naming one server must share a client")
	}
}

// Sharing must not extend across credentials or transport: a config with the
// wrong password reusing a working client would make the mistake invisible.
func TestClientsAreNotSharedAcrossIdentities(t *testing.T) {
	base := &goredis.Options{Network: "tcp", Addr: "localhost:6379", Username: "app", DB: 0}

	withPassword := *base
	withPassword.Password = "secret"

	otherDB := *base
	otherDB.DB = 1

	withTLS := *base
	withTLS.TLSConfig = &tls.Config{ServerName: "cache.example.com", MinVersion: tls.VersionTLS12}

	key := cacheKey(base)
	for name, o := range map[string]*goredis.Options{
		"password": &withPassword,
		"db":       &otherDB,
		"tls":      &withTLS,
	} {
		if cacheKey(o) == key {
			t.Errorf("a differing %s must not reuse the same client", name)
		}
	}

	// The same inputs still have to agree, or nothing would ever be reused.
	again := *base
	if cacheKey(&again) != key {
		t.Error("identical options must produce the same key")
	}
}
