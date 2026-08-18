package memory

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/config/source"
)

func loadedMemory(t *testing.T) *memory {
	t.Helper()

	m := NewLoader().(*memory)
	if err := m.Load(&testSource{data: []byte(`{"a":1}`)}); err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Load starts a watch goroutine per source. Without this it outlives the
	// test and keeps running while the rest of the package reports failures.
	t.Cleanup(func() { _ = m.Close() })
	return m
}

// Stop is reachable from two places at once — the caller's own defer and the
// config shutting its watcher down — and it used to select on the exit channel
// before closing it, so both callers could find it open.
func TestWatcherStopIsSafeUnderConcurrency(t *testing.T) {
	m := loadedMemory(t)

	for i := 0; i < 500; i++ {
		w, err := m.Watch("a")
		if err != nil {
			t.Fatalf("Watch: %v", err)
		}

		var wg sync.WaitGroup
		start := make(chan struct{})
		for j := 0; j < 16; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				if err := w.Stop(); err != nil {
					t.Errorf("Stop: %v", err)
				}
			}()
		}
		close(start)
		wg.Wait()
	}
}

// Stop also closed the updates channel, which update sends on. A watcher is
// removed from the list asynchronously, so a stop landing during a config
// change left the sender writing to a closed channel.
func TestStopDuringUpdateDoesNotPanic(t *testing.T) {
	m := loadedMemory(t)

	for i := 0; i < 500; i++ {
		w, err := m.Watch("a")
		if err != nil {
			t.Fatalf("Watch: %v", err)
		}

		var wg sync.WaitGroup
		wg.Add(2)
		start := make(chan struct{})
		go func() {
			defer wg.Done()
			<-start
			m.update()
		}()
		go func() {
			defer wg.Done()
			<-start
			_ = w.Stop()
		}()
		close(start)
		wg.Wait()
	}
}

type testSource struct {
	data        []byte
	unwatchable bool
}

func (s *testSource) Read() (*source.ChangeSet, error) {
	cs := &source.ChangeSet{
		Data:   s.data,
		Format: "json",
		Source: "test",
	}
	cs.Checksum = cs.Sum()
	return cs, nil
}

func (s *testSource) Write(*source.ChangeSet) error { return nil }

func (s *testSource) Watch() (source.Watcher, error) {
	if s.unwatchable {
		return nil, source.ErrWatcherStopped
	}
	return &testWatcher{stop: make(chan struct{})}, nil
}

func (s *testSource) String() string { return "test" }

// testWatcher reports nothing and blocks until it is stopped, which is what a
// quiet source looks like to the loader.
type testWatcher struct {
	stop chan struct{}
	once sync.Once
}

func (w *testWatcher) Next() (*source.ChangeSet, error) {
	<-w.stop
	return nil, source.ErrWatcherStopped
}

func (w *testWatcher) Stop() error {
	w.once.Do(func() { close(w.stop) })
	return nil
}

// A source that cannot be watched used to keep the loader'"'"'s goroutine retrying
// once a second forever: the exit check sat past the point the retry jumped
// back from, so Close could not reach it.
func TestCloseStopsAWatchThatCannotStart(t *testing.T) {
	m := NewLoader().(*memory)
	if err := m.Load(&testSource{data: []byte(`{"a":1}`), unwatchable: true}); err != nil {
		t.Fatalf("Load: %v", err)
	}

	before := runtime.NumGoroutine()
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// The goroutine is in a one second sleep at worst, so give it two.
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() >= before && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n >= before {
		t.Errorf("goroutine count %d did not fall below %d: the watch is still retrying", n, before)
	}
}

// Close carried the same select-then-close as the watcher, and the loader is
// closed both by its owner and by whatever is shutting the process down.
func TestLoaderCloseIsSafeUnderConcurrency(t *testing.T) {
	for i := 0; i < 500; i++ {
		m := NewLoader().(*memory)

		var wg sync.WaitGroup
		start := make(chan struct{})
		for j := 0; j < 16; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				if err := m.Close(); err != nil {
					t.Errorf("Close: %v", err)
				}
			}()
		}
		close(start)
		wg.Wait()
	}
}
