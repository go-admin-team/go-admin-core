package runtime

import (
	"sync"
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/storage"
	"github.com/go-admin-team/go-admin-core/storage/queue"
)

// Two callers asking for the queue have to reach the same one. When they did
// not, a consumer registered through one and a message published through the
// other went to unconnected queues and every message was lost in silence.
func TestQueuePrefixIsSharedWithoutConfiguration(t *testing.T) {
	app := NewConfig()

	got := make(chan storage.Messager, 1)
	app.GetQueuePrefix("consumer").Register("orders", func(m storage.Messager) error {
		got <- m
		return nil
	})

	producer := app.GetQueuePrefix("producer")
	if err := producer.Append(&queue.Message{Stream: "orders"}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	select {
	case <-got:
	case <-time.After(3 * time.Second):
		t.Fatal("a message published through one handle never reached a consumer registered through another")
	}
}

// fakeQueue is distinguishable from the fallback, which the real memory queue
// is not: both report "memory", so asserting on that would pass whether or not
// the configured adapter was used.
type fakeQueue struct{ storage.AdapterQueue }

func (fakeQueue) String() string { return "fake" }

// The configured adapter is what the queue getters must hand out, otherwise
// selecting Redis in the settings file changes nothing.
func TestConfiguredAdapterIsUsed(t *testing.T) {
	app := NewConfig()
	app.SetQueueAdapter(fakeQueue{})

	for name, q := range map[string]storage.AdapterQueue{
		"GetQueuePrefix":  app.GetQueuePrefix("x"),
		"GetQueueAdapter": app.GetQueueAdapter(),
	} {
		if q.String() != "fake" {
			t.Errorf("%s: got %q, want the configured adapter", name, q.String())
		}
	}
}

// Without configuration the getters fall back, and the fallback has to be the
// in-process queue rather than an error or a nil.
func TestFallbackIsTheMemoryQueue(t *testing.T) {
	app := NewConfig()

	if got := app.GetQueuePrefix("x").String(); got != "memory" {
		t.Errorf("got %q, want memory", got)
	}
}

// Reloading the configuration replaces the adapter while requests are in
// flight, so the field is read and written concurrently.
func TestAdapterSwapIsRaceFree(t *testing.T) {
	app := NewConfig()

	const rounds = 2000
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			app.SetQueueAdapter(queue.NewMemory(8))
			app.SetCacheAdapter(nil)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			_ = app.GetQueuePrefix("x")
			_ = app.GetCacheAdapterPrefix("x")
		}
	}()

	wg.Wait()
}
