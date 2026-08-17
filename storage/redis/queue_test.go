package redis_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/go-admin-team/go-admin-core/storage"
	"github.com/go-admin-team/go-admin-core/storage/cachetest"
	redisstore "github.com/go-admin-team/go-admin-core/storage/redis"
)

// The same suite that validates the in-memory implementation. One admin client
// serves every case, rather than a dial per case.
func TestRedisQueueConformance(t *testing.T) {
	url := redisURL(t)
	admin := adminClient(t, url)

	cachetest.RunQueue(t, func(t *testing.T) storage.Queue {
		flush(t, admin)

		q, err := redisstore.OpenQueue(context.Background(), url, redisstore.QueueOptions{
			// Short enough that a case which cancels within a few seconds does
			// not spend most of that time inside one blocking read.
			Block: 100 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("connect to Redis: %v", err)
		}
		return q
	})
}

// failing records every delivery and always reports an error, which is what
// keeps the entry pending and therefore eligible for a retry.
type failing struct {
	mu       sync.Mutex
	attempts []int
}

func (f *failing) handle(_ context.Context, m storage.Message) error {
	f.mu.Lock()
	f.attempts = append(f.attempts, m.Attempts)
	f.mu.Unlock()
	return errors.New("handler failed on purpose")
}

func (f *failing) seen() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]int, len(f.attempts))
	copy(out, f.attempts)
	return out
}

// A message whose handler keeps failing must be retried up to MaxAttempts and
// then left pending, so that it is still visible instead of silently gone.
func TestRetriesUntilMaxAttemptsThenLeavesPending(t *testing.T) {
	url := redisURL(t)
	admin := adminClient(t, url)
	flush(t, admin)

	const group = "retry-test"
	q, err := redisstore.OpenQueue(context.Background(), url, redisstore.QueueOptions{
		Group:        group,
		MaxAttempts:  3,
		Block:        50 * time.Millisecond,
		ClaimMinIdle: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("connect to Redis: %v", err)
	}
	defer q.Close()

	f := &failing{}
	if err := q.Subscribe("orders", f.handle); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := q.Start(ctx); err != nil {
			t.Errorf("Start: %v", err)
		}
	}()

	if err := q.Publish(context.Background(), storage.Message{Topic: "orders"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	deadline := time.After(10 * time.Second)
	for len(f.seen()) < 3 {
		select {
		case <-deadline:
			t.Fatalf("expected 3 attempts, saw %v", f.seen())
		case <-time.After(50 * time.Millisecond):
		}
	}

	// Long enough for several more sweeps, none of which may redeliver.
	time.Sleep(time.Second)

	if got := f.seen(); len(got) != 3 {
		t.Errorf("delivery must stop at MaxAttempts, saw attempts %v", got)
	} else {
		for i, n := range got {
			if n != i+1 {
				t.Errorf("attempt %d reported Attempts = %d, want %d", i, n, i+1)
			}
		}
	}

	cancel()
	<-done

	pending, err := admin.XPending(context.Background(), "orders", group).Result()
	if err != nil {
		t.Fatalf("XPENDING: %v", err)
	}
	if pending.Count != 1 {
		t.Errorf("a message that was given up on must stay pending, XPENDING reports %d", pending.Count)
	}
}

// roundTrip publishes one message and returns it as the handler saw it.
func roundTrip(t *testing.T, url string, values map[string]interface{}) storage.Message {
	t.Helper()

	q, err := redisstore.OpenQueue(context.Background(), url, redisstore.QueueOptions{
		Block: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("connect to Redis: %v", err)
	}
	t.Cleanup(func() { _ = q.Close() })

	got := make(chan storage.Message, 1)
	if err := q.Subscribe("orders", func(_ context.Context, m storage.Message) error {
		got <- m
		return nil
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = q.Start(ctx) }()

	if err := q.Publish(context.Background(), storage.Message{Topic: "orders", Values: values}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case m := <-got:
		return m
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the message")
		return storage.Message{}
	}
}

// Values travel as JSON, so a consumer sees the types JSON can express rather
// than the ones the publisher put in. The contract only promises strings; this
// records what actually happens to the rest.
func TestValuesTravelAsJSON(t *testing.T) {
	url := redisURL(t)
	flush(t, adminClient(t, url))

	m := roundTrip(t, url, map[string]interface{}{
		"id":    "42",
		"count": 7,
		"retry": true,
	})

	if m.Values["id"] != "42" {
		t.Errorf("a string must survive unchanged, got %#v", m.Values["id"])
	}
	if m.Values["count"] != float64(7) {
		t.Errorf("a number arrives as float64, got %#v", m.Values["count"])
	}
	if m.Values["retry"] != true {
		t.Errorf("a bool must survive unchanged, got %#v", m.Values["retry"])
	}
}

// An empty payload is carried by a reserved marker field, which must not eat a
// caller's field that happens to share its name.
func TestReservedFieldNameIsNotSwallowed(t *testing.T) {
	url := redisURL(t)
	flush(t, adminClient(t, url))

	m := roundTrip(t, url, map[string]interface{}{"go-admin:empty": "mine"})

	if m.Values["go-admin:empty"] != "mine" {
		t.Errorf("the caller's field was lost, got %#v", m.Values)
	}
}

// A field written by something other than this package is not JSON, so it
// cannot be decoded. It must reach the handler as the string Redis returned
// rather than being dropped.
func TestForeignFieldIsPassedThrough(t *testing.T) {
	url := redisURL(t)
	admin := adminClient(t, url)
	flush(t, admin)

	q, err := redisstore.OpenQueue(context.Background(), url, redisstore.QueueOptions{
		Block: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("connect to Redis: %v", err)
	}
	defer q.Close()

	got := make(chan storage.Message, 1)
	if err := q.Subscribe("orders", func(_ context.Context, m storage.Message) error {
		got <- m
		return nil
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = q.Start(ctx) }()

	// Written straight to the stream, bypassing Publish's JSON encoding.
	err = admin.XAdd(context.Background(), &goredis.XAddArgs{
		Stream: "orders",
		Values: map[string]interface{}{"raw": "not json at all"},
	}).Err()
	if err != nil {
		t.Fatalf("XADD: %v", err)
	}

	select {
	case m := <-got:
		if m.Values["raw"] != "not json at all" {
			t.Errorf("a foreign field must survive, got %#v", m.Values["raw"])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the message")
	}
}

// A caller-supplied client is shared, so Close must not shut it down.
func TestCloseLeavesBorrowedClientUsable(t *testing.T) {
	url := redisURL(t)

	cases := []struct {
		name string
		open func(*goredis.Client) interface{ Close() error }
	}{
		{"cache", func(c *goredis.Client) interface{ Close() error } {
			return redisstore.New(c)
		}},
		{"queue", func(c *goredis.Client) interface{ Close() error } {
			return redisstore.NewQueue(c, redisstore.QueueOptions{})
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := adminClient(t, url)

			if err := tc.open(client).Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := client.Ping(ctx).Err(); err != nil {
				t.Errorf("a borrowed client must stay usable after Close: %v", err)
			}
		})
	}
}
