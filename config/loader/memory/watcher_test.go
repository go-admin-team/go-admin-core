package memory

import (
	"sync"
	"testing"

	"github.com/go-admin-team/go-admin-core/config/source"
)

func loadedMemory(t *testing.T) *memory {
	t.Helper()

	m := NewLoader().(*memory)
	if err := m.Load(&testSource{data: []byte(`{"a":1}`)}); err != nil {
		t.Fatalf("Load: %v", err)
	}
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
	data []byte
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
	return nil, source.ErrWatcherStopped
}
func (s *testSource) String() string { return "test" }
