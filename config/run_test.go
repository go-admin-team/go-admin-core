package config

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/v2/config/loader"
	"github.com/go-admin-team/go-admin-core/v2/config/loader/memory"
)

// unwatchableLoader loads normally and refuses to be watched, which is the
// state run's retry loop was written for. It counts the attempts, and reports
// the first one, so the test can wait for the loop to be where it needs it
// rather than guessing at a duration.
type unwatchableLoader struct {
	loader.Loader
	tried    chan struct{}
	attempts atomic.Int64
}

func (l *unwatchableLoader) Watch(...string) (loader.Watcher, error) {
	l.attempts.Add(1)
	select {
	case l.tried <- struct{}{}:
	default:
	}
	return nil, errors.New("cannot watch")
}

// The exit check in run sits below the point the retry jumps back from, so a
// loader that cannot be watched used to keep the goroutine retrying once a
// second for the life of the process, closed or not.
//
// The question is asked of the loader rather than of runtime.NumGoroutine.
// That count is process-wide: every goroutine any other test in this package
// left behind is in it, so it answers "is anything at all still running"
// rather than "is this retry still running", and it went red on a busy machine
// while the retry under test had exited cleanly.
func TestCloseStopsARunThatCannotWatch(t *testing.T) {
	l := &unwatchableLoader{Loader: memory.NewLoader(), tried: make(chan struct{}, 1)}
	c, err := NewConfig(WithLoader(l))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}

	// Close has to arrive while run is in the retry, not before it gets there.
	select {
	case <-l.tried:
	case <-time.After(5 * time.Second):
		t.Fatal("run never attempted a watch")
	}

	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Read after Close, not before it. Taken before, a test goroutine that
	// happened to be descheduled for a second between the two would have the
	// retry that ran while it was away counted as one that ran after Close -
	// a red run for a loop that had in fact stopped, which is the failure
	// this test was rewritten to stop producing.
	at := l.attempts.Load()

	// The retry is one attempt a second, so what the assertion is made of is
	// silence: a loop that ignored Close would ask twice in this window. One
	// more attempt is allowed because Close can land on the same instant the
	// timer fires, and select is free to take the timer that once - but only
	// that once, since the next turn finds exit already closed.
	time.Sleep(2500 * time.Millisecond)
	if n := l.attempts.Load() - at; n > 1 {
		t.Errorf("the loader was asked to watch %d more times in the 2.5s after Close: run is still retrying", n)
	}
}
