package redis_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/go-admin-team/go-admin-core/v2/storage"
	redisstore "github.com/go-admin-team/go-admin-core/v2/storage/redis"
)

// These measure what moving off the in-memory backends costs. Redis is not
// optional once more than one instance runs - an in-memory cache is not shared
// and an in-memory queue is not durable - so the useful question is not whether
// it is slower but by how much, and against what.
//
// Read them next to the in-memory figures in storage/cache and storage/queue.
// Those are function calls; these are round trips, and the gap is the network,
// not the code. A server on loopback flatters Redis compared to one across a
// network, so treat these as the optimistic end.

// benchClient is separate from adminClient in cache_test.go because the pool
// has to be sized for the benchmark: with the default, parallel goroutines
// queue for a connection and the numbers describe that queue rather than Redis.
func benchClient(b *testing.B) *goredis.Client {
	b.Helper()
	opts, err := goredis.ParseURL(redisURL(b))
	if err != nil {
		b.Fatalf("parse REDIS_URL: %v", err)
	}
	opts.PoolSize = 128
	client := goredis.NewClient(opts)
	b.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(context.Background()).Err(); err != nil {
		b.Skipf("redis unreachable: %v", err)
	}
	return client
}

func BenchmarkRedisCacheGet(b *testing.B) {
	ctx := context.Background()
	c := redisstore.New(benchClient(b))
	if err := c.Set(ctx, "k", "v", time.Hour); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := c.Get(ctx, "k"); err != nil {
				b.Error(err)
				return
			}
		}
	})
}

func BenchmarkRedisCacheSet(b *testing.B) {
	ctx := context.Background()
	c := redisstore.New(benchClient(b))

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if err := c.Set(ctx, "k"+strconv.Itoa(i%64), "v", time.Hour); err != nil {
				b.Error(err)
				return
			}
			i++
		}
	})
}

// BenchmarkRedisCacheIncr is the counter the in-memory implementation had to be
// serialised by hand. Redis gets it from INCRBY, which is atomic on the server.
func BenchmarkRedisCacheIncr(b *testing.B) {
	ctx := context.Background()
	c := redisstore.New(benchClient(b))

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := c.Incr(ctx, "n", 1); err != nil {
				b.Error(err)
				return
			}
		}
	})
}

// BenchmarkRedisQueuePublish is the counterpart to the memory queue's Append.
// The two sit behind different contracts - this one takes a context and returns
// an error the caller can act on - but both answer the same question: what does
// handing off one message cost. Unlike the memory queue this cannot silently
// drop; a stream write either lands or reports why it did not.
func BenchmarkRedisQueuePublish(b *testing.B) {
	ctx := context.Background()
	client := benchClient(b)
	q := redisstore.NewQueue(client, redisstore.QueueOptions{
		Group:     "bench",
		KeyPrefix: "bench",
	})

	// Publish refuses a topic nobody consumes, so the group has to exist first.
	// Subscribe creates it without Start, which keeps the consumer loop out of
	// the measurement - this benchmarks the producer side only.
	if err := q.Subscribe("s", func(context.Context, storage.Message) error { return nil }); err != nil {
		b.Fatal(err)
	}

	values := map[string]interface{}{"k": "v"}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := q.Publish(ctx, storage.Message{Topic: "s", Values: values}); err != nil {
				b.Error(err)
				return
			}
		}
	})
	b.StopTimer()
	_ = client.FlushDB(ctx).Err()
}
