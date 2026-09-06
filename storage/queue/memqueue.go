package queue

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/go-admin-team/go-admin-core/v2/storage"
)

const defaultBuffer = 1024

// MemQueue is an in-process storage.Queue.
//
// Messages live only in memory, so nothing survives a restart and nothing is
// shared between instances. It is the default so that a single-instance
// deployment needs no broker; use a Redis-backed queue for anything else.
type MemQueue struct {
	mu       sync.RWMutex
	handlers map[string]storage.Handler
	closed   bool

	messages chan storage.Message
	seq      atomic.Int64

	// inFlight tracks deliveries so Close can wait for them.
	inFlight sync.WaitGroup

	started  bool
	stopOnce sync.Once
	stop     chan struct{}
	// drained is closed by Start when it returns. Close waits on it, because
	// inFlight alone cannot say whether the drain has begun: a Wait taken
	// before the drain's first delivery sees a count of zero and returns.
	//
	// It means "Start is finished", not "the buffer is empty". Start reached
	// through Close does drain first, but a Start that returns because its own
	// context was cancelled leaves whatever is buffered where it is - the
	// contract calls that cancellation rather than an error, and a caller who
	// cancels is asking to stop now.
	drained chan struct{}
}

var _ storage.Queue = (*MemQueue)(nil)

// String identifies the backend, which is what the deprecated AdapterQueue
// interface reports through storage.LegacyQueueAdapter.
func (q *MemQueue) String() string { return "memory" }

// NewMemQueue returns a queue buffering up to size messages. A size of zero or
// less uses the default.
func NewMemQueue(size int) *MemQueue {
	if size <= 0 {
		size = defaultBuffer
	}
	return &MemQueue{
		handlers: make(map[string]storage.Handler),
		messages: make(chan storage.Message, size),
		stop:     make(chan struct{}),
		drained:  make(chan struct{}),
	}
}

func (q *MemQueue) Subscribe(topic string, h storage.Handler) error {
	if h == nil {
		return storage.ErrNilHandler
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return storage.ErrQueueClosed
	}
	if q.started {
		return storage.ErrQueueAlreadyStarted
	}
	if _, exists := q.handlers[topic]; exists {
		return storage.ErrTopicAlreadySubscribed
	}
	q.handlers[topic] = h
	return nil
}

func (q *MemQueue) Publish(ctx context.Context, msg storage.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	q.mu.RLock()
	closed := q.closed
	_, known := q.handlers[msg.Topic]
	q.mu.RUnlock()

	if closed {
		return storage.ErrQueueClosed
	}
	if !known {
		return storage.ErrNoHandler
	}

	msg.ID = strconv.FormatInt(q.seq.Add(1), 10)
	if msg.Attempts == 0 {
		msg.Attempts = 1
	}

	select {
	case q.messages <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-q.stop:
		return storage.ErrQueueClosed
	}
}

func (q *MemQueue) Start(ctx context.Context) error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return storage.ErrQueueClosed
	}
	if q.started {
		q.mu.Unlock()
		return storage.ErrQueueAlreadyStarted
	}
	q.started = true
	q.mu.Unlock()

	// Signals the drain below has finished, which is what Close waits for.
	defer close(q.drained)

	for {
		select {
		case msg := <-q.messages:
			q.deliver(ctx, msg)
		case <-ctx.Done():
			// The contract states that cancellation is not an error.
			return nil
		case <-q.stop:
			// Drain what is already queued so Close does not lose messages
			// that were accepted before it was called.
			for {
				select {
				case msg := <-q.messages:
					q.deliver(ctx, msg)
				default:
					return nil
				}
			}
		}
	}
}

func (q *MemQueue) deliver(ctx context.Context, msg storage.Message) {
	q.mu.RLock()
	h := q.handlers[msg.Topic]
	// Deliberately not gated on q.closed. The drain in Start runs after Close
	// has set that flag, so refusing here made the drain take every remaining
	// message out of the buffer and throw it away - the opposite of what the
	// comment above it promised. Publish is where a closed queue stops
	// accepting; delivery of what it already accepted is what Close is for.
	if h == nil {
		q.mu.RUnlock()
		return
	}
	// What keeps this increment from racing Close's Wait is that Close waits
	// on q.drained first, and Start closes that only after its drain has
	// returned - so every deliver this queue will ever make has already
	// happened by then. It is no longer the lock: deliver stopped consulting
	// q.closed when that check turned out to be what discarded the drain.
	q.inFlight.Add(1)
	q.mu.RUnlock()
	defer q.inFlight.Done()

	// A handler error is reported through the queue's own error handling; this
	// implementation has no retry, so the message is dropped after one attempt.
	_ = h(ctx, msg)
}

func (q *MemQueue) Close() error {
	q.stopOnce.Do(func() {
		q.mu.Lock()
		q.closed = true
		started := q.started
		q.mu.Unlock()
		close(q.stop)

		// Only if something is consuming. Waiting on a queue nobody started
		// would block for ever, and there is nothing to drain in that case
		// anyway - no handler has ever seen a message.
		if started {
			<-q.drained
		}
	})

	// Start observes q.stop, drains what is already queued and returns on its
	// own; waiting for in-flight deliveries is what makes Close graceful.
	q.inFlight.Wait()
	return nil
}
