package queue

import (
	"github.com/go-admin-team/redisqueue/v2"
	"sync"

	"github.com/go-admin-team/go-admin-core/storage"
)

type Message struct {
	redisqueue.Message
	ErrorCount int
	mux        sync.RWMutex
}

func (m *Message) GetID() string {
	return m.ID
}

func (m *Message) GetStream() string {
	m.mux.Lock()
	defer m.mux.Unlock()
	return m.Stream
}

func (m *Message) GetValues() map[string]interface{} {
	m.mux.Lock()
	defer m.mux.Unlock()
	return m.Values
}

func (m *Message) SetID(id string) {
	m.ID = id
}

func (m *Message) SetStream(stream string) {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.Stream = stream
}

func (m *Message) SetValues(values map[string]interface{}) {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.Values = values
}

func (m *Message) GetPrefix() (prefix string) {
	m.mux.Lock()
	defer m.mux.Unlock()
	if m.Values == nil {
		return
	}
	v, _ := m.Values[storage.PrefixKey]
	prefix, _ = v.(string)
	return
}

func (m *Message) SetPrefix(prefix string) {
	m.mux.Lock()
	defer m.mux.Unlock()
	if m.Values == nil {
		m.Values = make(map[string]interface{})
	}
	m.Values[storage.PrefixKey] = prefix
}

func (m *Message) SetErrorCount(count int) {
	m.ErrorCount = count
}

func (m *Message) GetErrorCount() int {
	return m.ErrorCount
}
