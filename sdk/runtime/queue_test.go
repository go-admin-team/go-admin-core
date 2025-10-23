package runtime

import (
	"github.com/ht-qukuai-yjingy/go-admin-core/storage"
	"github.com/ht-qukuai-yjingy/go-admin-core/storage/queue"
	"reflect"
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
			if got := NewQueue(tt.args.prefix, tt.args.queue); !reflect.DeepEqual(got, tt.want) {
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
