package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

// LegacyQueueAdapter exposes a Queue through the older AdapterQueue interface,
// so a backend can be selected by configuration without changing the call
// sites that still take AdapterQueue.
//
// It cannot repair the older contract, only preserve it:
//
//   - Register returns nothing, so a rejected topic or an unreachable backend
//     can only be logged. Subscribe reports both; use it where that matters.
//   - ConsumerFunc takes no context, so a cancelled Start does not reach a
//     handler that is already running.
//   - Messager counts errors while Message counts deliveries. GetErrorCount is
//     therefore Attempts minus one, since the first delivery has not failed.
func LegacyQueueAdapter(q Queue) AdapterQueue {
	return &legacyQueueAdapter{inner: q}
}

type legacyQueueAdapter struct {
	inner Queue

	mu     sync.Mutex
	cancel context.CancelFunc
}

// String reports the backend's own identity where it has one. Operators read
// this to tell what is actually running, so the bridge must not stand in front
// of it.
func (l *legacyQueueAdapter) String() string {
	if s, ok := l.inner.(fmt.Stringer); ok {
		return s.String()
	}
	return "legacy"
}

func (l *legacyQueueAdapter) Append(message Messager) error {
	if message == nil {
		return errors.New("storage: nil message")
	}
	return l.inner.Publish(context.Background(), Message{
		Topic:  message.GetStream(),
		Values: message.GetValues(),
	})
}

func (l *legacyQueueAdapter) Register(name string, f ConsumerFunc) {
	if f == nil {
		slog.Error("queue: ignoring a nil consumer", "topic", name)
		return
	}

	err := l.inner.Subscribe(name, func(_ context.Context, msg Message) error {
		return f(&legacyMessage{msg: msg})
	})
	if err != nil {
		// The interface has no way to report this to the caller, which is one
		// of the reasons it is deprecated.
		slog.Error("queue: subscribe failed", "topic", name, "error", err)
	}
}

func (l *legacyQueueAdapter) Run() {
	ctx, cancel := context.WithCancel(context.Background())

	l.mu.Lock()
	if l.cancel != nil {
		l.mu.Unlock()
		cancel()
		slog.Error("queue: already running")
		return
	}
	l.cancel = cancel
	l.mu.Unlock()

	if err := l.inner.Start(ctx); err != nil {
		slog.Error("queue: stopped", "error", err)
	}
}

func (l *legacyQueueAdapter) Shutdown() {
	l.mu.Lock()
	cancel := l.cancel
	l.cancel = nil
	l.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if err := l.inner.Close(); err != nil {
		slog.Error("queue: close failed", "error", err)
	}
}

// legacyMessage presents a Message through the older Messager interface.
type legacyMessage struct {
	msg Message
}

var _ Messager = (*legacyMessage)(nil)

func (m *legacyMessage) GetID() string      { return m.msg.ID }
func (m *legacyMessage) SetID(id string)    { m.msg.ID = id }
func (m *legacyMessage) GetStream() string  { return m.msg.Topic }
func (m *legacyMessage) SetStream(s string) { m.msg.Topic = s }

func (m *legacyMessage) GetValues() map[string]interface{} { return m.msg.Values }

func (m *legacyMessage) SetValues(v map[string]interface{}) { m.msg.Values = v }

func (m *legacyMessage) GetPrefix() string {
	if m.msg.Values == nil {
		return ""
	}
	prefix, _ := m.msg.Values[PrefixKey].(string)
	return prefix
}

func (m *legacyMessage) SetPrefix(prefix string) {
	if m.msg.Values == nil {
		m.msg.Values = make(map[string]interface{})
	}
	m.msg.Values[PrefixKey] = prefix
}

// GetErrorCount reports one less than Attempts: the older interface counted
// failures, the new one counts deliveries, and the first delivery has not
// failed yet.
func (m *legacyMessage) GetErrorCount() int { return m.msg.Attempts - 1 }

func (m *legacyMessage) SetErrorCount(count int) { m.msg.Attempts = count + 1 }
