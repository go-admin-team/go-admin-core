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
// state run's retry loop was written for.
type unwatchableLoader struct {
	loader.Loader
}

func (l *unwatchableLoader) Watch(...string) (loader.Watcher, error) {
	return nil, errors.New("cannot watch")
}

// The exit check in run sits below the point the retry jumps back from, so a
// loader that cannot be watched used to keep the goroutine retrying once a
// second for the life of the process, closed or not.
func TestCloseStopsARunThatCannotWatch(t *testing.T) {
	before := runtime.NumGoroutine()

	c, err := NewConfig(WithLoader(&unwatchableLoader{Loader: memory.NewLoader()}))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}

	// Let run reach the retry rather than racing its first iteration.
	time.Sleep(50 * time.Millisecond)

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
