package queue

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/v2/storage"
)

func memMsg(stream string) storage.Messager {
	m := new(Message)
	m.SetStream(stream)
	m.SetValues(map[string]interface{}{"a": "b"})
	return m
}

// Shutdown has to deliver what the queue already accepted before it returns.
//
// It used to release Run's wait group and return immediately, which read as
// harmless in a test - the consumer goroutines were still alive, because
// nothing stopped them, so they carried on and the messages arrived. In a
// process it is total loss: Shutdown returning is the last thing that happens
// before the process exits, and those goroutines go with it.
//
// So the count is read the instant Shutdown returns. Sleeping first measures
// the leak, not the fix.
func TestShutdownDeliversWhatIsStillBuffered(t *testing.T) {
	m := NewMemory(64)

	var delivered atomic.Int64
	m.Register("t", func(storage.Messager) error {
		time.Sleep(20 * time.Millisecond) // slow enough that a backlog forms
		delivered.Add(1)
		return nil
	})

	const n = 20
	for i := 0; i < n; i++ {
		if err := m.Append(memMsg("t")); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	go m.Run()

	m.Shutdown()

	if got := delivered.Load(); got != n {
		t.Errorf("Shutdown returned with %d of %d messages undelivered", int64(n)-got, n)
	}
}

// Nothing may be delivered after Shutdown returns.
//
// This is the observable half of "the consumers are gone": a goroutine that
// survived would keep reading its channel, and a later Append would reach a
// handler on a queue the caller believes is shut. It is asserted through
// behaviour rather than through a goroutine count, which is process-wide and
// answers "is anything still running" rather than "is this still running".
func TestNothingIsDeliveredAfterShutdownReturns(t *testing.T) {
	m := NewMemory(64)

	var delivered atomic.Int64
	m.Register("t", func(storage.Messager) error {
		delivered.Add(1)
		return nil
	})
	go m.Run()

	if err := m.Append(memMsg("t")); err != nil {
		t.Fatalf("append: %v", err)
	}
	m.Shutdown()
	at := delivered.Load()

	// A closed queue refuses new work, which is the first half of why nothing
	// more can arrive.
	if err := m.Append(memMsg("t")); !errors.Is(err, storage.ErrQueueClosed) {
		t.Errorf("Append after Shutdown returned %v, want %v", err, storage.ErrQueueClosed)
	}

	time.Sleep(300 * time.Millisecond)
	if now := delivered.Load(); now != at {
		t.Errorf("%d messages were delivered after Shutdown returned", now-at)
	}
}

// A consumer that survived Shutdown would still be reading its channel, and
// that is directly observable from inside the package: put a message into the
// channel by hand and see whether anything takes it.
//
// This replaces a runtime.NumGoroutine comparison, which was in this file
// briefly and failed - the count is process-wide, so goroutines other tests in
// this binary left behind sit in the baseline and it never comes back down. It
// answers "is anything still running" rather than "is this still running".
func TestShutdownLeavesNoConsumerReadingTheChannel(t *testing.T) {
	m := NewMemory(8)

	var delivered atomic.Int64
	m.Register("t", func(storage.Messager) error {
		delivered.Add(1)
		return nil
	})
	go m.Run()
	m.Shutdown()

	at := delivered.Load()

	// Straight into the channel, bypassing Append, which now refuses. Only a
	// consumer goroutine still in its loop could take this.
	ch := m.queueFor("t")
	select {
	case ch <- memMsg("t"):
	default:
		t.Fatal("could not place a message in the channel; the test proves nothing")
	}

	time.Sleep(300 * time.Millisecond)
	if now := delivered.Load(); now != at {
		t.Errorf("a consumer took %d message(s) after Shutdown: it is still running", now-at)
	}
}

// Shutting a queue down twice must not panic on a channel already closed, and
// must not wait a second time.
func TestShutdownIsIdempotent(t *testing.T) {
	m := NewMemory(8)
	m.Register("t", func(storage.Messager) error { return nil })
	go m.Run()

	m.Shutdown()
	done := make(chan struct{})
	go func() { m.Shutdown(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the second Shutdown blocked")
	}
}

// Registering on a queue that is already shut does nothing rather than start a
// goroutine nothing would ever stop.
func TestRegisterAfterShutdownStartsNothing(t *testing.T) {
	m := NewMemory(8)
	m.Shutdown()

	var delivered atomic.Int64
	m.Register("t", func(storage.Messager) error {
		delivered.Add(1)
		return nil
	})
	_ = m.Append(memMsg("t")) // refused, but assert on the handler anyway

	time.Sleep(200 * time.Millisecond)
	if delivered.Load() != 0 {
		t.Error("a consumer registered after Shutdown received a message")
	}
}
