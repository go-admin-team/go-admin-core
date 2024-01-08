package queue

import (
	"github.com/go-admin-team/go-admin-core/storage"
	"github.com/go-admin-team/redisqueue/v2"
	"github.com/redis/go-redis/v9"
)

// NewRedis redis模式
func NewRedis(
	producerOptions *redisqueue.ProducerOptions,
	consumerOptions *redisqueue.ConsumerOptions,
) (*Redis, error) {
	var err error
	r := &Redis{}
	r.producer, err = r.newProducer(producerOptions)
	if err != nil {
		return nil, err
	}
	r.consumer, err = r.newConsumer(consumerOptions)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// Redis cache implement
type Redis struct {
	client   *redis.Client
	consumer *redisqueue.Consumer
	producer *redisqueue.Producer
}

func (Redis) String() string {
	return "redis"
}

func (r *Redis) newConsumer(options *redisqueue.ConsumerOptions) (*redisqueue.Consumer, error) {
	if options == nil {
		options = &redisqueue.ConsumerOptions{}
	}
	return redisqueue.NewConsumerWithOptions(options)
}

func (r *Redis) newProducer(options *redisqueue.ProducerOptions) (*redisqueue.Producer, error) {
	if options == nil {
		options = &redisqueue.ProducerOptions{}
	}
	return redisqueue.NewProducerWithOptions(options)
}

func (r *Redis) Append(message storage.Messager) error {
	err := r.producer.Enqueue(&redisqueue.Message{
		ID:     message.GetID(),
		Stream: message.GetStream(),
		Values: message.GetValues(),
	})
	return err
}

func (r *Redis) Register(name string, f storage.ConsumerFunc) {
	r.consumer.Register(name, func(message *redisqueue.Message) error {
		m := new(Message)
		m.SetValues(message.Values)
		m.SetStream(message.Stream)
		m.SetID(message.ID)
		return f(m)
	})
}

func (r *Redis) Run() {
	r.consumer.Run()
}

func (r *Redis) Shutdown() {
	r.consumer.Shutdown()
}
