package runtime

import (
	"fmt"
	"github.com/go-admin-team/redisqueue/v2"
	"github.com/redis/go-redis/v9"
	"reflect"
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/storage"
	"github.com/go-admin-team/go-admin-core/storage/queue"
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
	client := redis.NewClient(&redis.Options{})
	q, err := queue.NewRedis(&redisqueue.ProducerOptions{
		StreamMaxLength:      100,
		ApproximateMaxLength: true,
		RedisClient:          client,
	}, &redisqueue.ConsumerOptions{
		VisibilityTimeout: 60 * time.Second,
		BlockingTimeout:   5 * time.Second,
		ReclaimInterval:   1 * time.Second,
		BufferSize:        100,
		Concurrency:       10,
		RedisClient:       client,
	})
	if err != nil {
		t.Error(err)
		return
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"test0",
			fields{
				prefix: "",
				queue:  q,
			},
			args{
				name: "operate_log_queue",
				f: func(m storage.Messager) error {
					fmt.Println(m.GetValues())
					return nil
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//var e storage.AdapterQueue
			e := &Queue{
				prefix: tt.fields.prefix,
				queue:  tt.fields.queue,
			}
			e.Register(tt.args.name, tt.args.f)
			e.Run()
		})
	}
}
