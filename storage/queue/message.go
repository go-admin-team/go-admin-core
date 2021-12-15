package queue

import (
	"github.com/robinjoseph08/redisqueue/v2"

	"github.com/go-admin-team/go-admin-core/storage"
)

type Message struct {
	redisqueue.Message
	ErrorCount int
}

func (m *Message) GetID() string {
	return m.ID
}

func (m *Message) GetStream() string {
	return m.Stream
}

func (m *Message) GetValues() map[string]interface{} {
	return m.Values
}

func (m *Message) SetID(id string) {
	m.ID = id
}

func (m *Message) SetStream(stream string) {
	m.Stream = stream
}

func (m *Message) SetValues(values map[string]interface{}) {
	m.Values = values
}

func (m *Message) GetPrefix() (prefix string) {
	if m.Values == nil {
		return
	}
	v, _ := m.Values[storage.PrefixKey]
	prefix, _ = v.(string)
	return
}

func (m *Message) SetPrefix(prefix string) {
	if m.Values == nil {
		m.Values = make(map[string]interface{})
	}
	m.Values[storage.PrefixKey] = prefix
}

func (m *Message) SetErrorCount() {
	m.ErrorCount = m.ErrorCount + 1
}

func (m *Message) GetErrorCount() int {
	return m.ErrorCount
}
