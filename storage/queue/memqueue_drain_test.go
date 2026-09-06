package queue

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/v2/storage"
)

// startedQueue returns a queue with a counting handler and a Start already in
// its loop, so that a test can publish into a running queue rather than into a
// buffer nobody is reading.
func startedQueue(t *testing.T, size int) (*MemQueue, *atomic.Int64) {
	t.Helper()
	q := NewMemQueue(size)

	var delivered atomic.Int64
	running := make(chan struct{})
	var once sync.Once
	if err := q.Subscribe("t", func(context.Context, storage.Message) error {
		delivered.Add(1)
		once.Do(func() { close(running) })
		return nil
	}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	go func() { _ = q.Start(context.Background()) }()

	if err := q.Publish(context.Background(), storage.Message{Topic: "t"}); err != nil {
		t.Fatalf("publish the first message: %v", err)
	}
	select {
	case <-running:
	case <-time.After(5 * time.Second):
		t.Fatal("Start never consumed anything")
	}
	return q, &delivered
}

// Close has to deliver what the queue already accepted.
//
// It did not. Close set the closed flag and only then signalled the drain, and
// deliver returned early whenever that flag was set - so the drain took every
// remaining message out of the buffer and dropped it, which is the opposite of
// what the comment above the drain said it was for. Twenty published, one
// delivered.
//
// Publish is where a closed queue refuses work. Delivering what it accepted
// before that is the whole job of Close.
func TestCloseDeliversWhatIsStillBuffered(t *testing.T) {
	q, delivered := startedQueue(t, 64)

	const buffered = 20
	for i := 0; i < buffered; i++ {
		if err := q.Publish(context.Background(), storage.Message{Topic: "t"}); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}

	if err := q.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// No sleep: Close returning is the claim under test. If it can return
	// before the drain has finished, this reads a short count.
	if got := delivered.Load(); got != buffered+1 {
		t.Errorf("Close returned with %d of %d messages delivered", got, buffered+1)
	}
}

// Close must not report success before the drain has run. inFlight alone
// cannot carry that: a Wait taken before the drain's first delivery sees a
// count of zero and returns, so the messages land after the caller has been
// told the queue is closed - or not at all, if the process is exiting.
func TestCloseWaitsForTheDrainRatherThanForInFlightAlone(t *testing.T) {
	q, delivered := startedQueue(t, 64)

	for i := 0; i < 50; i++ {
		if err := q.Publish(context.Background(), storage.Message{Topic: "t"}); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}
	_ = q.Close()

	// Read immediately and again after a pause. If Close waited properly the
	// two are equal; if it returned early, the second is larger.
	immediately := delivered.Load()
	time.Sleep(300 * time.Millisecond)
	if later := delivered.Load(); later != immediately {
		t.Errorf("%d more messages were delivered after Close returned: it did not wait for the drain", later-immediately)
	}
}

// Closing a queue nobody started must not block. There is nothing to drain -
// no handler has seen a message - and waiting for a Start that will never come
// would hang the shutdown it is part of.
func TestCloseDoesNotBlockOnAQueueThatWasNeverStarted(t *testing.T) {
	q := NewMemQueue(8)
	if err := q.Subscribe("t", func(context.Context, storage.Message) error { return nil }); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- q.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Close: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close blocked on a queue that was never started")
	}
}

// Start after Close is refused rather than quietly starting a consumer on a
// queue that will never be closed again - the flag Close sets is the same one
// Publish reads, and a Start that ignored it would leave the drain signal
// already spent.
func TestStartAfterCloseIsRefused(t *testing.T) {
	q := NewMemQueue(8)
	if err := q.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := q.Start(context.Background()); !errors.Is(err, storage.ErrQueueClosed) {
		t.Errorf("Start after Close returned %v, want %v", err, storage.ErrQueueClosed)
	}
}
