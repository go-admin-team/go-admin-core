package cachetest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/storage"
)

// QueueFactory builds a Queue for a single case.
type QueueFactory func(t *testing.T) storage.Queue

// RunQueue executes the queue conformance suite against newQueue.
func RunQueue(t *testing.T, newQueue QueueFactory) {
	t.Helper()

	tests := []struct {
		name string
		fn   func(*testing.T, QueueFactory)
	}{
		{"DeliversPublishedMessage", testDeliversPublishedMessage},
		{"PreservesValues", testPreservesValues},
		{"StringValuesRoundTrip", testStringValuesRoundTrip},
		{"EmptyPayloadIsDelivered", testEmptyPayloadIsDelivered},
		{"AssignsID", testAssignsID},
		{"RoutesByTopic", testRoutesByTopic},
		{"RejectsDuplicateSubscription", testRejectsDuplicateSubscription},
		{"RejectsNilHandler", testRejectsNilHandler},
		{"PublishWithoutHandlerFails", testPublishWithoutHandlerFails},
		{"AttemptsStartAtOne", testAttemptsStartAtOne},
		{"StartReturnsOnContextCancel", testStartReturnsOnContextCancel},
		{"SubscribeAfterStartIsRejected", testSubscribeAfterStartIsRejected},
		{"SecondStartIsRejected", testSecondStartIsRejected},
		{"CloseIsIdempotent", testQueueCloseIsIdempotent},
		{"PublishAfterCloseFails", testPublishAfterCloseFails},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) { tc.fn(t, newQueue) })
	}
}

// started runs q.Start in the background and returns a stop function.
func started(t *testing.T, q storage.Queue) func() {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := q.Start(ctx); err != nil {
			t.Errorf("Start returned %v; cancellation must be a clean exit", err)
		}
	}()

	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("Start did not return after the context was cancelled")
		}
	}
}

// collector records delivered messages.
type collector struct {
	mu   sync.Mutex
	msgs []storage.Message
	got  chan struct{}
}

func newCollector() *collector {
	return &collector{got: make(chan struct{}, 64)}
}

func (c *collector) handle(_ context.Context, m storage.Message) error {
	c.mu.Lock()
	c.msgs = append(c.msgs, m)
	c.mu.Unlock()
	select {
	case c.got <- struct{}{}:
	default:
	}
	return nil
}

func (c *collector) await(t *testing.T, n int) []storage.Message {
	t.Helper()

	deadline := time.After(3 * time.Second)
	for {
		c.mu.Lock()
		got := len(c.msgs)
		c.mu.Unlock()
		if got >= n {
			break
		}
		select {
		case <-c.got:
		case <-deadline:
			t.Fatalf("timed out waiting for %d message(s), got %d", n, got)
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]storage.Message, len(c.msgs))
	copy(out, c.msgs)
	return out
}

func testDeliversPublishedMessage(t *testing.T, newQueue QueueFactory) {
	q := newQueue(t)
	defer q.Close()

	c := newCollector()
	if err := q.Subscribe("orders", c.handle); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	stop := started(t, q)
	defer stop()

	if err := q.Publish(context.Background(), storage.Message{Topic: "orders"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	c.await(t, 1)
}

func testPreservesValues(t *testing.T, newQueue QueueFactory) {
	q := newQueue(t)
	defer q.Close()

	c := newCollector()
	_ = q.Subscribe("orders", c.handle)
	stop := started(t, q)
	defer stop()

	_ = q.Publish(context.Background(), storage.Message{
		Topic:  "orders",
		Values: map[string]interface{}{"id": "42", "kind": "refund"},
	})

	got := c.await(t, 1)[0]
	if got.Values["id"] != "42" || got.Values["kind"] != "refund" {
		t.Errorf("values were not preserved: %v", got.Values)
	}
}

func testAssignsID(t *testing.T, newQueue QueueFactory) {
	q := newQueue(t)
	defer q.Close()

	c := newCollector()
	_ = q.Subscribe("orders", c.handle)
	stop := started(t, q)
	defer stop()

	_ = q.Publish(context.Background(), storage.Message{Topic: "orders"})

	if got := c.await(t, 1)[0]; got.ID == "" {
		t.Error("the queue must assign an ID on publish")
	}
}

func testRoutesByTopic(t *testing.T, newQueue QueueFactory) {
	q := newQueue(t)
	defer q.Close()

	a, b := newCollector(), newCollector()
	_ = q.Subscribe("a", a.handle)
	_ = q.Subscribe("b", b.handle)
	stop := started(t, q)
	defer stop()

	_ = q.Publish(context.Background(), storage.Message{Topic: "b"})

	b.await(t, 1)

	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.msgs) != 0 {
		t.Errorf("a message reached the wrong topic: %v", a.msgs)
	}
}

// Registration must be rejected rather than silently replacing the handler,
// which is how the previous Register behaved.
func testRejectsDuplicateSubscription(t *testing.T, newQueue QueueFactory) {
	q := newQueue(t)
	defer q.Close()

	noop := func(context.Context, storage.Message) error { return nil }
	if err := q.Subscribe("orders", noop); err != nil {
		t.Fatalf("first Subscribe: %v", err)
	}

	err := q.Subscribe("orders", noop)
	if !errors.Is(err, storage.ErrTopicAlreadySubscribed) {
		t.Errorf("second Subscribe: got %v, want ErrTopicAlreadySubscribed", err)
	}
}

// A nil handler would make Publish succeed while delivery silently drops the
// message, since Publish only checks that the topic is registered.
func testRejectsNilHandler(t *testing.T, newQueue QueueFactory) {
	q := newQueue(t)
	defer q.Close()

	err := q.Subscribe("orders", nil)
	if !errors.Is(err, storage.ErrNilHandler) {
		t.Errorf("got %v, want ErrNilHandler", err)
	}
}

// Only strings are promised to survive; a broker-backed queue has to serialise
// the payload and an in-process one does not.
func testStringValuesRoundTrip(t *testing.T, newQueue QueueFactory) {
	q := newQueue(t)
	defer q.Close()

	c := newCollector()
	if err := q.Subscribe("orders", c.handle); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	stop := started(t, q)
	defer stop()

	err := q.Publish(context.Background(), storage.Message{
		Topic:  "orders",
		Values: map[string]interface{}{"id": "42", "note": ""},
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	got := c.await(t, 1)[0]
	if got.Values["id"] != "42" {
		t.Errorf("id: got %#v, want the string 42", got.Values["id"])
	}
	if got.Values["note"] != "" {
		t.Errorf("an empty string is a legal value, got %#v", got.Values["note"])
	}
}

// A message with no values is still a message, and some backends cannot store
// an entry that carries no field at all.
func testEmptyPayloadIsDelivered(t *testing.T, newQueue QueueFactory) {
	q := newQueue(t)
	defer q.Close()

	c := newCollector()
	_ = q.Subscribe("orders", c.handle)
	stop := started(t, q)
	defer stop()

	if err := q.Publish(context.Background(), storage.Message{Topic: "orders"}); err != nil {
		t.Fatalf("Publish with no values: %v", err)
	}

	if got := c.await(t, 1)[0]; len(got.Values) != 0 {
		t.Errorf("an empty payload must stay empty, got %#v", got.Values)
	}
}

// A topic registered after Start would be accepted by Publish, because the
// backend can see the subscription, while nothing reads it. Rejecting the
// subscription is what keeps that from becoming a silent backlog.
func testSubscribeAfterStartIsRejected(t *testing.T, newQueue QueueFactory) {
	q := newQueue(t)
	defer q.Close()

	c := newCollector()
	if err := q.Subscribe("orders", c.handle); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	stop := started(t, q)
	defer stop()

	// As above, a delivered message proves Start is running.
	if err := q.Publish(context.Background(), storage.Message{Topic: "orders"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	c.await(t, 1)

	err := q.Subscribe("late", func(context.Context, storage.Message) error { return nil })
	if !errors.Is(err, storage.ErrQueueAlreadyStarted) {
		t.Errorf("Subscribe after Start: got %v, want ErrQueueAlreadyStarted", err)
	}
}

// Start is not restartable; a second call must say so rather than running a
// second read loop against the same handlers.
func testSecondStartIsRejected(t *testing.T, newQueue QueueFactory) {
	q := newQueue(t)
	defer q.Close()

	c := newCollector()
	if err := q.Subscribe("orders", c.handle); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	stop := started(t, q)
	defer stop()

	// A delivered message proves the first Start is past the guard. Without
	// this, the two calls race and either one may be the one that wins.
	if err := q.Publish(context.Background(), storage.Message{Topic: "orders"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	c.await(t, 1)

	// Already cancelled, so an implementation that wrongly runs a second read
	// loop fails the assertion instead of hanging the suite.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := q.Start(ctx); !errors.Is(err, storage.ErrQueueAlreadyStarted) {
		t.Errorf("second Start: got %v, want ErrQueueAlreadyStarted", err)
	}
}

func testPublishWithoutHandlerFails(t *testing.T, newQueue QueueFactory) {
	q := newQueue(t)
	defer q.Close()

	err := q.Publish(context.Background(), storage.Message{Topic: "nobody-listens"})
	if !errors.Is(err, storage.ErrNoHandler) {
		t.Errorf("got %v, want ErrNoHandler: a dropped message must not look like a success", err)
	}
}

func testAttemptsStartAtOne(t *testing.T, newQueue QueueFactory) {
	q := newQueue(t)
	defer q.Close()

	c := newCollector()
	_ = q.Subscribe("orders", c.handle)
	stop := started(t, q)
	defer stop()

	_ = q.Publish(context.Background(), storage.Message{Topic: "orders"})

	if got := c.await(t, 1)[0]; got.Attempts != 1 {
		t.Errorf("Attempts on first delivery: got %d, want 1", got.Attempts)
	}
}

func testStartReturnsOnContextCancel(t *testing.T, newQueue QueueFactory) {
	q := newQueue(t)
	defer q.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- q.Start(ctx) }()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Start returned %v; cancellation is not an error", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("Start did not return after cancellation")
	}
}

func testQueueCloseIsIdempotent(t *testing.T, newQueue QueueFactory) {
	q := newQueue(t)

	if err := q.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := q.Close(); err != nil {
		t.Errorf("second Close must be a no-op, got %v", err)
	}
}

func testPublishAfterCloseFails(t *testing.T, newQueue QueueFactory) {
	q := newQueue(t)
	_ = q.Subscribe("orders", func(context.Context, storage.Message) error { return nil })
	if err := q.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	err := q.Publish(context.Background(), storage.Message{Topic: "orders"})
	if !errors.Is(err, storage.ErrQueueClosed) {
		t.Errorf("Publish after Close: got %v, want ErrQueueClosed", err)
	}
}
