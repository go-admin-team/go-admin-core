package redis_test

import (
	"context"
	"os"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/go-admin-team/go-admin-core/storage"
	"github.com/go-admin-team/go-admin-core/storage/cachetest"
	redisstore "github.com/go-admin-team/go-admin-core/storage/redis"
)

// redisURL is where the tests look for a server. CI provides one through a
// service container; locally the tests skip unless REDIS_URL is set.
func redisURL(t *testing.T) string {
	t.Helper()

	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL is not set; skipping the Redis conformance suite")
	}
	return url
}

// The same suite that validates the in-memory implementation. Two independent
// implementations agreeing on it is what makes the contract meaningful.
func TestRedisConformance(t *testing.T) {
	url := redisURL(t)

	cachetest.Run(t, func(t *testing.T) storage.Cache {
		ctx := context.Background()

		c, err := redisstore.Open(ctx, url)
		if err != nil {
			t.Fatalf("connect to Redis: %v", err)
		}

		// Each case starts from an empty keyspace, otherwise counters and
		// expiry assertions leak into one another.
		opts, err := goredis.ParseURL(url)
		if err != nil {
			t.Fatalf("parse REDIS_URL: %v", err)
		}
		admin := goredis.NewClient(opts)
		defer admin.Close()
		if err := admin.FlushDB(ctx).Err(); err != nil {
			t.Fatalf("flush: %v", err)
		}

		return c
	})
}

// A caller-supplied client is shared, so Close must not shut it down.
func TestCloseLeavesBorrowedClientUsable(t *testing.T) {
	url := redisURL(t)

	opts, err := goredis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse REDIS_URL: %v", err)
	}
	client := goredis.NewClient(opts)
	defer client.Close()

	c := redisstore.New(client)
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Errorf("a borrowed client must stay usable after Close: %v", err)
	}
}
