package queue

import (
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/robinjoseph08/redisqueue/v2"

	"github.com/go-admin-team/go-admin-core/storage"
)

func TestMemory_Append(t *testing.T) {
	type fields struct {
		items *sync.Map
		queue *sync.Map
		wait  sync.WaitGroup
		mutex sync.RWMutex
	}
	type args struct {
		name    string
		message storage.Messager
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"test01",
			fields{},
			args{
				name: "test",
				message: &Message{redisqueue.Message{
					ID:     "",
					Stream: "test",
					Values: map[string]interface{}{
						"key": "value",
					},
				}},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMemory(100)
			if err := m.Append(tt.args.message); (err != nil) != tt.wantErr {
				t.Errorf("Append() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemory_Register(t *testing.T) {
	log.SetFlags(19)
	type fields struct {
		items *sync.Map
		queue *sync.Map
		wait  sync.WaitGroup
		mutex sync.RWMutex
	}
	type args struct {
		name string
		f    storage.ConsumerFunc
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"test01",
			fields{},
			args{
				name: "test",
				f: func(message storage.Messager) error {
					fmt.Println(message.GetValues())
					return nil
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMemory(100)
			m.Register(tt.name, tt.args.f)
			if err := m.Append(&Message{redisqueue.Message{
				ID:     "",
				Stream: "test",
				Values: map[string]interface{}{
					"key": "value",
				},
			}}); err != nil {
				t.Error(err)
				return
			}
			go func() {
				m.Run()
			}()
			time.Sleep(3 * time.Second)
			m.Shutdown()
		})
	}
}
