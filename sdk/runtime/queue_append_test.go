package runtime

import (
	"testing"

	"github.com/go-admin-team/go-admin-core/v2/storage/queue"
)

// Append tags the message with the queue's prefix so that a consumer can tell
// which tenant it came from. It read the values map, created one when it was
// nil, wrote the prefix into that, and then published the message — which
// still had no values. A message published without values lost its tenant.
func TestAppendTagsAMessageThatHasNoValues(t *testing.T) {
	q := NewQueue("tenant-a", queue.NewMemory(10))

	m := &queue.Message{}
	if m.GetValues() != nil {
		t.Fatal("this test needs a message with no values")
	}

	if err := q.Append(m); err != nil {
		t.Fatalf("Append: %v", err)
	}

	if got := m.GetPrefix(); got != "tenant-a" {
		t.Errorf("prefix is %q, want tenant-a: the tag never reached the message", got)
	}
}

// The ordinary case, so that a fix cannot work by ignoring what was there.
func TestAppendKeepsExistingValues(t *testing.T) {
	q := NewQueue("tenant-b", queue.NewMemory(10))

	m := &queue.Message{}
	m.SetValues(map[string]interface{}{"id": "7"})

	if err := q.Append(m); err != nil {
		t.Fatalf("Append: %v", err)
	}

	values := m.GetValues()
	if values["id"] != "7" {
		t.Errorf("id is %v, want 7", values["id"])
	}
	if got := m.GetPrefix(); got != "tenant-b" {
		t.Errorf("prefix is %q, want tenant-b", got)
	}
}
