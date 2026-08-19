package runtime

import (
	"github.com/go-admin-team/go-admin-core/v2/storage"
	"github.com/go-admin-team/go-admin-core/v2/storage/queue"
	"testing"
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

func TestQueue_Register(t *testing.T) {
	type fields struct {
		prefix string
		queue  storage.AdapterQueue
	}
	type args struct {
		name string
		f    storage.ConsumerFunc
	}

}
