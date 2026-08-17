package storage

import (
	"context"
	"errors"
	"io"
)

var (
	// ErrQueueClosed is returned by operations on a closed Queue.
	ErrQueueClosed = errors.New("storage: queue closed")

	// ErrTopicAlreadySubscribed reports a second handler for one topic.
	// Registration is rejected rather than silently replacing the first, which
	// is what the previous Register did.
	ErrTopicAlreadySubscribed = errors.New("storage: topic already subscribed")

	// ErrNoHandler reports a message published to a topic nobody consumes.
	ErrNoHandler = errors.New("storage: no handler for topic")

	// ErrNilHandler reports a subscription with nothing to deliver to.
	ErrNilHandler = errors.New("storage: nil handler")

	// ErrQueueAlreadyStarted reports a second call to Start, or a Subscribe
	// that arrived too late to take effect.
	ErrQueueAlreadyStarted = errors.New("storage: queue already started")
)

// Message is a queued message.
//
// It is a struct, not an interface: the previous Messager was ten getters and
// setters with a single implementation, so the indirection bought nothing.
// Values is a map rather than a byte slice to match the field model of Redis
// streams, which is the intended backend.
type Message struct {
	// ID is assigned by the queue on publish. It is ignored on input.
	ID string

	// Topic routes the message to a handler.
	Topic string

	// Values carries the payload.
	//
	// Only string values are guaranteed to arrive unchanged. Everything else is
	// undefined across implementations, because a broker-backed queue has to
	// serialise the map and an in-process one does not: the Redis queue returns
	// a published int as a float64, the memory queue returns the int. Use
	// strings, or encode the payload yourself.
	Values map[string]interface{}

	// Attempts counts deliveries of this message, starting at 1. A handler can
	// use it to give up on a message that keeps failing.
	Attempts int
}

// Handler processes one message. Returning an error marks the delivery as
// failed; whether that leads to a retry is the implementation's decision, but
// Attempts must reflect it.
type Handler func(ctx context.Context, msg Message) error

// Queue is the contract for a message queue.
type Queue interface {
	// Publish enqueues a message. The topic must have a subscriber, otherwise
	// ErrNoHandler is returned rather than dropping the message silently.
	Publish(ctx context.Context, msg Message) error

	// Subscribe registers the handler for a topic. It must be called before
	// Start, because an implementation may fix its set of topics there and a
	// late subscription would then let Publish succeed with nothing reading it.
	//
	// Where more than one rule is broken at once, every implementation has to
	// answer alike: a bad argument outranks the queue's state, and among
	// states the terminal one outranks the rest. So ErrNilHandler, then
	// ErrQueueClosed, then ErrQueueAlreadyStarted, then
	// ErrTopicAlreadySubscribed.
	Subscribe(topic string, h Handler) error

	// Start begins consuming and blocks until ctx is done or an unrecoverable
	// error occurs. A context cancellation is not an error.
	Start(ctx context.Context) error

	// Close stops accepting new messages and waits for in-flight deliveries to
	// finish. It is idempotent. Bound the wait with a context of your own if
	// you need to.
	io.Closer
}
