package config

import (
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/config/loader"
	"github.com/go-admin-team/go-admin-core/config/loader/memory"
)

// unwatchableLoader loads normally and refuses to be watched, which is the
// state run's retry loop was written for. It reports each attempt so the test
// can wait for the loop to be where it needs it rather than guessing at a
// duration.
type unwatchableLoader struct {
	loader.Loader
	tried chan struct{}
}

func (l *unwatchableLoader) Watch(...string) (loader.Watcher, error) {
	select {
	case l.tried <- struct{}{}:
	default:
	}
	return nil, errors.New("cannot watch")
}

// The exit check in run sits below the point the retry jumps back from, so a
// loader that cannot be watched used to keep the goroutine retrying once a
// second for the life of the process, closed or not.
func TestCloseStopsARunThatCannotWatch(t *testing.T) {
	before := runtime.NumGoroutine()

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

	deadline := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > before {
		t.Errorf("goroutine count %d did not return to %d: run is still retrying", n, before)
	}
}
