package redis_test

import (
	"context"
	"os"
	"testing"

	goredis "github.com/redis/go-redis/v9"

	"github.com/go-admin-team/go-admin-core/v2/storage"
	"github.com/go-admin-team/go-admin-core/v2/storage/cachetest"
	redisstore "github.com/go-admin-team/go-admin-core/v2/storage/redis"
)

// redisURL is where the tests look for a server. CI provides one through a
// service container; locally the tests skip unless REDIS_URL is set.
func redisURL(t testing.TB) string {
	t.Helper()

	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL is not set; skipping the Redis conformance suite")
	}
	return url
}

// adminClient is a plain client for the test's own bookkeeping, closed with the
// test that asked for it.
func adminClient(t *testing.T, url string) *goredis.Client {
	t.Helper()

	opts, err := goredis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse REDIS_URL: %v", err)
	}
	client := goredis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// flush empties the keyspace so that keys, streams and consumer groups left by
// one case cannot reach the next.
func flush(t *testing.T, admin *goredis.Client) {
	t.Helper()

	if err := admin.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("flush: %v", err)
	}
}

// The same suite that validates the in-memory implementation. Two independent
// implementations agreeing on it is what makes the contract meaningful.
func TestRedisConformance(t *testing.T) {
	url := redisURL(t)
	admin := adminClient(t, url)

	cachetest.Run(t, func(t *testing.T) storage.Cache {
		// Each case starts from an empty keyspace, otherwise counters and
		// expiry assertions leak into one another.
		flush(t, admin)

		c, err := redisstore.Open(context.Background(), url)
		if err != nil {
			t.Fatalf("connect to Redis: %v", err)
		}
		return c
	})
}
