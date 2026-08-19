package storage_test

import (
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/v2/storage"
	"github.com/go-admin-team/go-admin-core/v2/storage/queue"
)

func newLegacyQueue(t *testing.T) storage.AdapterQueue {
	t.Helper()
	return storage.LegacyQueueAdapter(queue.NewMemQueue(64))
}

// The point of the adapter: a call site written against AdapterQueue keeps
// working when the backend underneath it changes.
func TestLegacyQueueAdapterDelivers(t *testing.T) {
	q := newLegacyQueue(t)

	got := make(chan storage.Messager, 1)
	q.Register("orders", func(m storage.Messager) error {
		got <- m
		return nil
	})

	go q.Run()
	defer q.Shutdown()

	msg := &queue.Message{Stream: "orders", Values: map[string]interface{}{"id": "42"}}
	if err := q.Append(msg); err != nil {
		t.Fatalf("Append: %v", err)
	}

	select {
	case m := <-got:
		if m.GetStream() != "orders" {
			t.Errorf("stream: got %q, want orders", m.GetStream())
		}
		if m.GetValues()["id"] != "42" {
			t.Errorf("values were not preserved: %v", m.GetValues())
		}
		if m.GetID() == "" {
			t.Error("the queue must assign an ID")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for the message")
	}
}

// Append still reports an unconsumed topic, which is the one thing the older
// interface can express.
func TestLegacyQueueAdapterReportsUnknownTopic(t *testing.T) {
	q := newLegacyQueue(t)
	defer q.Shutdown()

	err := q.Append(&queue.Message{Stream: "nobody-listens"})
	if err == nil {
		t.Error("Append to an unconsumed topic must fail rather than drop the message")
	}
}

func TestLegacyQueueAdapterRejectsNilMessage(t *testing.T) {
	q := newLegacyQueue(t)
	defer q.Shutdown()

	if err := q.Append(nil); err == nil {
		t.Error("a nil message must be rejected instead of panicking")
	}
}

// Shutdown must be safe whether or not Run was ever called, since the older
// interface gives no way to tell.
func TestLegacyQueueAdapterShutdownIsIdempotent(t *testing.T) {
	q := newLegacyQueue(t)

	q.Shutdown()
	q.Shutdown()
}

// Messager counts failures, Message counts deliveries. A first delivery has not
// failed yet, so GetErrorCount must report zero.
func TestLegacyQueueAdapterErrorCountOffset(t *testing.T) {
	q := newLegacyQueue(t)

	got := make(chan int, 1)
	q.Register("orders", func(m storage.Messager) error {
		got <- m.GetErrorCount()
		return nil
	})

	go q.Run()
	defer q.Shutdown()

	if err := q.Append(&queue.Message{Stream: "orders"}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	select {
	case n := <-got:
		if n != 0 {
			t.Errorf("GetErrorCount on a first delivery: got %d, want 0", n)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for the message")
	}
}

// A duplicate topic cannot be reported through Register, so the first handler
// has to stay in place rather than being silently replaced.
func TestLegacyQueueAdapterKeepsTheFirstHandler(t *testing.T) {
	q := newLegacyQueue(t)

	first := make(chan struct{}, 1)
	second := make(chan struct{}, 1)
	q.Register("orders", func(storage.Messager) error { first <- struct{}{}; return nil })
	q.Register("orders", func(storage.Messager) error { second <- struct{}{}; return nil })

	go q.Run()
	defer q.Shutdown()

	if err := q.Append(&queue.Message{Stream: "orders"}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	select {
	case <-first:
	case <-second:
		t.Error("the second Register must not replace the first handler")
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for the message")
	}
}

// Run blocks until the queue stops, which is what "go q.Run()" at every call
// site relies on.
func TestLegacyQueueAdapterRunReturnsOnShutdown(t *testing.T) {
	q := newLegacyQueue(t)

	done := make(chan struct{})
	go func() {
		defer close(done)
		q.Run()
	}()

	// Give Run a moment to reach the read loop before stopping it.
	time.Sleep(50 * time.Millisecond)
	q.Shutdown()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Error("Run did not return after Shutdown")
	}
}
