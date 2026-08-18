package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/go-admin-team/go-admin-core/storage"
)

const (
	defaultGroup        = "go-admin"
	defaultMaxAttempts  = 3
	defaultBlock        = time.Second
	defaultClaimMinIdle = 30 * time.Second
	defaultBatch        = 16
)

// consumerSeq keeps the generated consumer names distinct when one process
// builds several queues. Two consumers sharing a name would take over each
// other's pending deliveries.
var consumerSeq atomic.Int64

// QueueOptions configures a Queue. The zero value is usable.
type QueueOptions struct {
	// Group is the consumer group. Every instance of one application must use
	// the same value; a distinct group receives its own copy of every message.
	Group string

	// Consumer identifies this instance inside the group and must be unique.
	// The default combines the hostname with the process id.
	Consumer string

	// KeyPrefix is prepended to the topic to form the stream key, so that
	// several applications can share one Redis database.
	KeyPrefix string

	// MaxAttempts bounds redelivery. A message delivered this many times
	// without being acknowledged is left in the pending list rather than
	// dropped, so that it stays visible to XPENDING.
	MaxAttempts int

	// Block is how long one read waits for new messages before looping. It also
	// bounds how long Start takes to notice a cancellation, because a blocking
	// read is not interrupted by one.
	Block time.Duration

	// ClaimMinIdle is how long a delivery must go unacknowledged before another
	// consumer may take it over, and also the interval between retry sweeps.
	// Set it above the running time of the slowest handler.
	ClaimMinIdle time.Duration

	// Batch is how many messages a single read or sweep may return.
	Batch int64
}

func (o QueueOptions) withDefaults() QueueOptions {
	if o.Group == "" {
		o.Group = defaultGroup
	}
	if o.Consumer == "" {
		o.Consumer = defaultConsumer()
	}
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = defaultMaxAttempts
	}
	if o.Block <= 0 {
		o.Block = defaultBlock
	}
	if o.ClaimMinIdle <= 0 {
		o.ClaimMinIdle = defaultClaimMinIdle
	}
	if o.Batch <= 0 {
		o.Batch = defaultBatch
	}
	return o
}

func defaultConsumer() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	return fmt.Sprintf("%s-%d-%d", host, os.Getpid(), consumerSeq.Add(1))
}

// Queue is a storage.Queue backed by Redis streams.
//
// Consumer groups are what make it usable from more than one instance: every
// instance reads under the same group, so each message is handled once, and a
// delivery whose handler fails stays pending until it is retried here or taken
// over by another instance. Delivery is therefore at least once; a handler must
// tolerate seeing the same message twice.
//
// Topic names are expected to be a fixed, finite set. Publish caches the topics
// it has confirmed a consumer group for, so generating topic names per user or
// per tenant would grow that cache without bound.
type Queue struct {
	client goredis.UniversalClient
	// owned reports whether Close should shut the client down. A client
	// supplied by the caller may be shared and is left alone.
	owned bool
	opts  QueueOptions

	mu       sync.RWMutex
	handlers map[string]storage.Handler
	// groups caches topics known to have a consumer group in Redis, whether it
	// was created here or by another instance.
	groups map[string]struct{}
	closed bool

	// inFlight tracks deliveries so Close can wait for them.
	inFlight sync.WaitGroup

	// started is guarded by mu because marking the queue started and freezing
	// its topic set have to be one step.
	started bool

	// stopCtx is cancelled by Close, so a running Start unblocks even when its
	// own context is still live.
	stopCtx    context.Context
	stopCancel context.CancelFunc
}

var _ storage.Queue = (*Queue)(nil)

// NewQueue returns a Queue backed by client. Close does not shut client down,
// because the caller may share it with other components.
func NewQueue(client goredis.UniversalClient, opts QueueOptions) *Queue {
	return newQueue(client, false, opts)
}

// OpenQueue connects using a Redis URL, for example
// redis://user:password@localhost:6379/0. The returned Queue owns the
// connection and closes it.
func OpenQueue(ctx context.Context, url string, opts QueueOptions) (*Queue, error) {
	client, err := dial(ctx, url)
	if err != nil {
		return nil, err
	}
	return newQueue(client, true, opts), nil
}

func newQueue(client goredis.UniversalClient, owned bool, opts QueueOptions) *Queue {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &Queue{
		client:     client,
		owned:      owned,
		opts:       opts.withDefaults(),
		handlers:   make(map[string]storage.Handler),
		groups:     make(map[string]struct{}),
		stopCtx:    stopCtx,
		stopCancel: stopCancel,
	}
}

// String identifies the backend, which is what the deprecated AdapterQueue
// interface reports through storage.LegacyQueueAdapter.
func (q *Queue) String() string { return "redis" }

func (q *Queue) stream(topic string) string { return q.opts.KeyPrefix + topic }

func (q *Queue) topic(stream string) string {
	return strings.TrimPrefix(stream, q.opts.KeyPrefix)
}

func (q *Queue) Subscribe(topic string, h storage.Handler) error {
	if h == nil {
		return storage.ErrNilHandler
	}

	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return storage.ErrQueueClosed
	}
	if q.started {
		q.mu.Unlock()
		return storage.ErrQueueAlreadyStarted
	}
	if _, exists := q.handlers[topic]; exists {
		q.mu.Unlock()
		return storage.ErrTopicAlreadySubscribed
	}
	// The topic is claimed before the round trip so that a concurrent Subscribe
	// loses, then given back if the group cannot be created.
	q.handlers[topic] = h
	q.mu.Unlock()

	// The group is created here rather than in Start, so that a message
	// published in between is kept for this subscriber instead of being lost.
	if err := q.ensureGroup(context.Background(), topic); err != nil {
		q.mu.Lock()
		delete(q.handlers, topic)
		q.mu.Unlock()
		return err
	}

	q.mu.Lock()
	q.groups[topic] = struct{}{}
	q.mu.Unlock()
	return nil
}

// begin marks the queue started and returns the topics it will serve. The two
// happen under one lock, so a Subscribe cannot slip between them and register
// a topic the read loop will never look at.
func (q *Queue) begin() ([]string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.started {
		return nil, storage.ErrQueueAlreadyStarted
	}
	q.started = true

	topics := make([]string, 0, len(q.handlers))
	for t := range q.handlers {
		topics = append(topics, t)
	}
	return topics, nil
}

// ensureGroup creates the consumer group, treating an existing one as success.
func (q *Queue) ensureGroup(ctx context.Context, topic string) error {
	// MKSTREAM creates the stream when it is missing, and "$" starts the group
	// at the current end so a new subscriber does not replay the whole history.
	err := q.client.XGroupCreateMkStream(ctx, q.stream(topic), q.opts.Group, "$").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

// isMissingGroup reports the error Redis returns for a read against a group
// that does not exist. It has no dedicated type in the client, so the reply is
// matched on its prefix, which is what NOGROUP is for.
func isMissingGroup(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "NOGROUP")
}

// groupExists asks Redis whether anyone consumes the topic.
func (q *Queue) groupExists(ctx context.Context, topic string) (bool, error) {
	groups, err := q.client.XInfoGroups(ctx, q.stream(topic)).Result()
	if err != nil {
		// A stream that was never created has no group either. Redis reports
		// this as an ordinary error rather than a nil reply.
		if errors.Is(err, goredis.Nil) || strings.Contains(err.Error(), "no such key") {
			return false, nil
		}
		return false, err
	}
	for _, g := range groups {
		if g.Name == q.opts.Group {
			return true, nil
		}
	}
	return false, nil
}

func (q *Queue) Publish(ctx context.Context, msg storage.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	q.mu.RLock()
	closed := q.closed
	_, known := q.groups[msg.Topic]
	q.mu.RUnlock()

	if closed {
		return storage.ErrQueueClosed
	}
	if !known {
		// The subscriber may live in another process, so a missing local
		// registration is not evidence that nobody consumes the topic. The
		// answer is cached, so this costs one round trip per topic.
		ok, err := q.groupExists(ctx, msg.Topic)
		if err != nil {
			return err
		}
		if !ok {
			return storage.ErrNoHandler
		}
		q.mu.Lock()
		q.groups[msg.Topic] = struct{}{}
		q.mu.Unlock()
	}

	values, err := encodeValues(msg.Values)
	if err != nil {
		return err
	}

	// The stream assigns the ID, which is what the consumer sees on delivery.
	return q.client.XAdd(ctx, &goredis.XAddArgs{
		Stream: q.stream(msg.Topic),
		Values: values,
	}).Err()
}

func (q *Queue) Start(ctx context.Context) error {
	topics, err := q.begin()
	if err != nil {
		return err
	}

	// Registered first so that it runs last: the cancel below has to happen
	// before this Wait, otherwise returning on a read error would block here
	// with reclaimLoop still watching a live context.
	var wg sync.WaitGroup
	defer wg.Wait()

	// Close has to unblock the read as well, so both signals feed one context.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer context.AfterFunc(q.stopCtx, cancel)()

	if len(topics) == 0 {
		// XREADGROUP needs at least one stream, so there is nothing to do but
		// wait for the caller to give up. Said out loud, because a queue that
		// consumes nothing is almost always a wiring mistake — Subscribe has to
		// happen before Start, and a caller that starts first gets silence.
		slog.Warn("queue: started with no subscriptions, nothing will be consumed",
			"group", q.opts.Group)
		<-ctx.Done()
		return nil
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		q.reclaimLoop(ctx, topics)
	}()

	// XREADGROUP takes every key first and every start ID second. One read
	// covers every topic, so an idle queue costs one round trip per Block
	// rather than one per topic.
	args := make([]string, 0, len(topics)*2)
	for _, t := range topics {
		args = append(args, q.stream(t))
	}
	for range topics {
		args = append(args, ">")
	}

	for {
		res, err := q.client.XReadGroup(ctx, &goredis.XReadGroupArgs{
			Group:    q.opts.Group,
			Consumer: q.opts.Consumer,
			Streams:  args,
			Count:    q.opts.Batch,
			Block:    q.opts.Block,
		}).Result()
		switch {
		case errors.Is(err, goredis.Nil):
			// The block elapsed with nothing new.
			continue
		case isMissingGroup(err):
			// The group can be gone for two reasons: a Subscribe that has not
			// finished creating it yet, or an operator deleting the stream
			// while this is running. Neither is a reason to stop consuming
			// forever, which is what returning here used to do.
			for _, t := range topics {
				if err := q.ensureGroup(ctx, t); err != nil && ctx.Err() == nil {
					slog.Warn("queue: could not recreate the consumer group",
						"topic", t, "error", err)
				}
			}
			continue
		case err != nil:
			if ctx.Err() != nil {
				// Cancellation, whether from the caller or from Close, is a
				// clean exit rather than a failure.
				return nil
			}
			return err
		}

		for _, s := range res {
			topic := q.topic(s.Stream)
			for _, m := range s.Messages {
				// A message read with ">" has been delivered exactly once.
				q.deliver(ctx, topic, m, 1)
			}
		}
	}
}

// reclaimLoop retries deliveries that were never acknowledged, which is how a
// message survives a handler error or the instance that held it going away.
func (q *Queue) reclaimLoop(ctx context.Context, topics []string) {
	t := time.NewTicker(q.opts.ClaimMinIdle)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for _, topic := range topics {
				q.reclaim(ctx, topic)
			}
		}
	}
}

func (q *Queue) reclaim(ctx context.Context, topic string) {
	stream := q.stream(topic)

	pending, err := q.client.XPendingExt(ctx, &goredis.XPendingExtArgs{
		Stream: stream,
		Group:  q.opts.Group,
		Idle:   q.opts.ClaimMinIdle,
		Start:  "-",
		End:    "+",
		Count:  q.opts.Batch,
	}).Result()
	if err != nil {
		// Transient; the next sweep tries again.
		return
	}

	ids := make([]string, 0, len(pending))
	attempts := make(map[string]int, len(pending))
	for _, p := range pending {
		if int(p.RetryCount) >= q.opts.MaxAttempts {
			// Given up on, but deliberately not acknowledged: the entry stays
			// in the pending list where XPENDING still shows it. Acknowledging
			// here would make an unhandled message look handled.
			continue
		}
		ids = append(ids, p.ID)
		// XCLAIM increments the delivery counter, so this will be attempt n+1.
		attempts[p.ID] = int(p.RetryCount) + 1
	}
	if len(ids) == 0 {
		return
	}

	// One XCLAIM for the whole batch. An entry another consumer took first is
	// simply absent from the result. XAUTOCLAIM would be shorter still but
	// cannot filter on delivery count, so it would drag back the entries that
	// have exhausted MaxAttempts.
	claimed, err := q.client.XClaim(ctx, &goredis.XClaimArgs{
		Stream:   stream,
		Group:    q.opts.Group,
		Consumer: q.opts.Consumer,
		MinIdle:  q.opts.ClaimMinIdle,
		Messages: ids,
	}).Result()
	if err != nil {
		return
	}

	for _, m := range claimed {
		q.deliver(ctx, topic, m, attempts[m.ID])
	}
}

func (q *Queue) deliver(ctx context.Context, topic string, m goredis.XMessage, attempts int) {
	q.mu.RLock()
	h := q.handlers[topic]
	if h == nil || q.closed {
		q.mu.RUnlock()
		return
	}
	// Registered while holding the lock Close uses to publish q.closed, so the
	// counter can never be incremented after Close began waiting on it.
	q.inFlight.Add(1)
	q.mu.RUnlock()
	defer q.inFlight.Done()

	err := h(ctx, storage.Message{
		ID:       m.ID,
		Topic:    topic,
		Values:   decodeValues(m.Values),
		Attempts: attempts,
	})
	if err != nil {
		// Left unacknowledged on purpose: the pending entry is what makes the
		// retry possible, here or on another instance.
		return
	}

	// Acknowledged outside ctx, which a shutdown may already have cancelled. A
	// handled message that stays pending would be delivered a second time. The
	// call is bounded by the client's own read timeout.
	//
	// A failed acknowledgement only costs a redelivery, which the at-least-once
	// guarantee already allows for.
	_ = q.client.XAck(context.WithoutCancel(ctx), q.stream(topic), q.opts.Group, m.ID).Err()
}

// Close stops accepting messages, waits for in-flight deliveries and, when it
// owns the client, shuts it down. Nothing is drained: whatever is unhandled
// stays pending in Redis for the next instance to pick up, which is the point
// of using consumer groups.
func (q *Queue) Close() error {
	q.mu.Lock()
	first := !q.closed
	q.closed = true
	q.mu.Unlock()

	q.stopCancel()
	q.inFlight.Wait()

	if first && q.owned {
		return q.client.Close()
	}
	return nil
}

// emptyField stands in for a message with no values. XADD rejects an entry
// that carries no field at all, and a message whose payload is empty is still
// a message worth delivering.
//
// Its value is the empty string, which json.Marshal cannot produce, so a
// caller's own field of the same name is never mistaken for the marker.
const emptyField = "go-admin:empty"

// encodeValues renders each value as JSON, because a stream field is a string.
func encodeValues(in map[string]interface{}) (map[string]interface{}, error) {
	if len(in) == 0 {
		return map[string]interface{}{emptyField: ""}, nil
	}

	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("storage: encode value %q: %w", k, err)
		}
		out[k] = string(b)
	}
	return out, nil
}

func decodeValues(in map[string]interface{}) map[string]interface{} {
	if len(in) == 1 {
		if v, ok := in[emptyField]; ok && v == "" {
			return map[string]interface{}{}
		}
	}

	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		s, ok := v.(string)
		if !ok {
			out[k] = v
			continue
		}

		var decoded interface{}
		if err := json.Unmarshal([]byte(s), &decoded); err != nil {
			// Written by something other than this package; pass it through as
			// the string Redis returned.
			out[k] = s
			continue
		}
		out[k] = decoded
	}
	return out
}
