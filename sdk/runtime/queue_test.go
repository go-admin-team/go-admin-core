package runtime

import (
	"github.com/go-admin-team/go-admin-core/v2/storage"
	"github.com/go-admin-team/go-admin-core/v2/storage/queue"
	"testing"
	"time"
)

func TestNewMemoryQueue(t *testing.T) {
	type args struct {
		prefix string
		queue  storage.AdapterQueue
	}
	tests := []struct {
		name string
		args args
		want storage.AdapterQueue
	}{
		{
			"test0",
			args{
				prefix: "",
				queue:  nil,
			},
			queue.NewMemory(100),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewQueue(tt.args.prefix, tt.args.queue)
			// 比较字符串表示（类型），而不是 DeepEqual（因为是不同的实例）
			if got.String() != tt.want.String() {
				t.Errorf("NewQueue() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Register forwards to the backing adapter, so what this checks is that a
// consumer registered through the wrapper receives what is published through
// it. The test was a stub — two type declarations and no body — which reads as
// coverage from the outside.
func TestQueue_Register(t *testing.T) {
	q := NewQueue("tenant", queue.NewMemory(10))

	got := make(chan storage.Messager, 1)
	q.Register("orders", func(m storage.Messager) error {
		got <- m
		return nil
	})

	m := &queue.Message{}
	m.SetValues(map[string]interface{}{"id": "1"})
	m.SetStream("orders")
	if err := q.Append(m); err != nil {
		t.Fatalf("Append: %v", err)
	}

	go q.Run()
	t.Cleanup(func() { q.Shutdown() })

	select {
	case received := <-got:
		if received.GetPrefix() != "tenant" {
			t.Errorf("prefix is %q, want tenant", received.GetPrefix())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the registered consumer never received the message")
	}
}
