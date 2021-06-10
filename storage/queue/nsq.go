/*
 * @Author: lwnmengjing
 * @Date: 2021/5/30 7:30 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/5/30 7:30 下午
 */

package queue

import (
	"github.com/go-admin-team/go-admin-core/storage"
	json "github.com/json-iterator/go"
	"github.com/nsqio/go-nsq"
)

// NewNSQ nsq模式 只能监听一个channel
func NewNSQ(addresses []string, cfg *nsq.Config, channelPrefix string) (*NSQ, error) {
	n := &NSQ{
		addresses:     addresses,
		cfg:           cfg,
		channelPrefix: channelPrefix,
	}
	var err error
	n.producer, err = n.newProducer()
	return n, err
}

type NSQ struct {
	addresses     []string
	cfg           *nsq.Config
	producer      *nsq.Producer
	consumer      *nsq.Consumer
	channelPrefix string
}

// String 字符串类型
func (NSQ) String() string {
	return "nsq"
}

// switchAddress ⚠️生产环境至少配置三个节点
func (e *NSQ) switchAddress() {
	if len(e.addresses) > 1 {
		e.addresses[0], e.addresses[len(e.addresses)-1] =
			e.addresses[1],
			e.addresses[0]
	}
}

func (e *NSQ) newProducer() (*nsq.Producer, error) {
	if e.cfg == nil {
		e.cfg = nsq.NewConfig()
	}
	return nsq.NewProducer(e.addresses[0], e.cfg)
}

func (e *NSQ) newConsumer(topic string, h nsq.Handler) (err error) {
	if e.cfg == nil {
		e.cfg = nsq.NewConfig()
	}
	if e.consumer == nil {
		e.consumer, err = nsq.NewConsumer(topic, e.channelPrefix+topic, e.cfg)
		if err != nil {
			return err
		}
	}
	e.consumer.AddHandler(h)
	err = e.consumer.ConnectToNSQDs(e.addresses)

	return err
}

// Append 消息入生产者
func (e *NSQ) Append(message storage.Messager) error {
	rb, err := json.Marshal(message.GetValues())
	if err != nil {
		return err
	}
	return e.producer.Publish(message.GetStream(), rb)
}

// Register 监听消费者
func (e *NSQ) Register(name string, f storage.ConsumerFunc) {
	h := &nsqConsumerHandler{f}
	err := e.newConsumer(name, h)
	if err != nil {
		//目前不支持动态注册
		panic(err)
	}
}

func (e *NSQ) Run() {
}

func (e *NSQ) Shutdown() {
	if e.producer != nil {
		e.producer.Stop()
	}
	if e.consumer != nil {
		e.consumer.Stop()
	}
}

type nsqConsumerHandler struct {
	f storage.ConsumerFunc
}

func (e nsqConsumerHandler) HandleMessage(message *nsq.Message) error {
	m := new(Message)
	data := make(map[string]interface{})
	err := json.Unmarshal(message.Body, &data)
	if err != nil {
		return err
	}
	m.SetValues(data)
	return e.f(m)
}
