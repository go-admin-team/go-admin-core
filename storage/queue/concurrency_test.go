package queue

import (
	"io"
	"log"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/v2/storage"
)

// The in-memory queue carries the login and operation logs of a server running
// without redis, which is the default. Producers are request goroutines and the
// consumer is registered at startup, so the two overlap in the ordinary case
// rather than an exotic one.

// TestRegisterAndAppendAgreeOnOneQueue pins the fix for the queue-creation
// race. Both sides used to Load, miss, and Store their own channel; whichever
// stored second won, and anything already written to the loser was never read.
// Registering while the first messages arrive lost about one in seven.
func TestRegisterAndAppendAgreeOnOneQueue(t *testing.T) {
	const attempts = 200
	lost := 0

	for i := 0; i < attempts; i++ {
		m := NewMemory(100)
		delivered := make(chan struct{}, 4)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			m.Register("s", func(storage.Messager) error {
				delivered <- struct{}{}
				return nil
			})
		}()
		go func() {
			defer wg.Done()
			_ = m.Append(newTestMessage("s"))
		}()
		wg.Wait()

		select {
		case <-delivered:
		case <-time.After(50 * time.Millisecond):
			lost++
		}
	}

	if lost > 0 {
		t.Errorf("%d of %d messages never reached the consumer; Register and Append "+
			"disagreed about which channel is the queue", lost, attempts)
	}
}

// TestConcurrentAppendersShareOneQueue covers the producer-only case: many
// request goroutines writing the same stream must all reach the one consumer.
func TestConcurrentAppendersShareOneQueue(t *testing.T) {
	m := NewMemory(4096)

	var received atomic.Int64
	m.Register("s", func(storage.Messager) error {
		received.Add(1)
		return nil
	})

	const producers, each = 32, 50
	var sent atomic.Int64
	var wg sync.WaitGroup
	wg.Add(producers)
	for i := 0; i < producers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < each; j++ {
				if err := m.Append(newTestMessage("s")); err == nil {
					sent.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	// The consumer runs on its own goroutine; give it a bounded chance to drain
	// rather than sleeping a fixed amount and hoping.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && received.Load() < sent.Load() {
		time.Sleep(5 * time.Millisecond)
	}

	if got, want := received.Load(), sent.Load(); got != want {
		t.Errorf("consumer saw %d of %d accepted messages", got, want)
	}
}

// --- throughput ---------------------------------------------------------

// benchAppend measures Append with a consumer draining behind it, and reports
// the share of messages the queue refused. Append is non-blocking: a full queue
// drops the message and returns an error rather than parking the caller, so
// throughput alone would look excellent while the log silently went missing.
// The drop rate is the number worth reading.
func benchAppend(b *testing.B, poolNum uint, streams int) {
	// Every drop is reported through the standard logger. At benchmark rates
	// that is millions of lines competing for stderr, which would be what the
	// numbers below measured. Worth noting for production too: a saturated
	// queue turns into a log storm exactly when the process is already behind.
	prev := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(prev)

	m := NewMemory(poolNum)
	for i := 0; i < streams; i++ {
		m.Register("s"+strconv.Itoa(i), func(storage.Messager) error { return nil })
	}

	var dropped atomic.Int64

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if err := m.Append(newTestMessage("s" + strconv.Itoa(i%streams))); err != nil {
				dropped.Add(1)
			}
			i++
		}
	})
	b.StopTimer()

	if b.N > 0 {
		b.ReportMetric(float64(dropped.Load())*100/float64(b.N), "%dropped")
	}
}

func BenchmarkMemoryQueueAppend(b *testing.B)            { benchAppend(b, 100, 1) }
func BenchmarkMemoryQueueAppendLargePool(b *testing.B)   { benchAppend(b, 10000, 1) }
func BenchmarkMemoryQueueAppendMultiStream(b *testing.B) { benchAppend(b, 10000, 8) }
