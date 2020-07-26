package rocketmq

import(
	"errors"
	"context"
	"sync"
	"github.com/google/uuid"
	"github.com/micro/go-micro/v2/broker"
	"github.com/micro/go-micro/v2/config/cmd"
	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/producer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
)


type rBroker struct {
	addrs []string

	p rocketmq.Producer

	sc []rocketmq.PushConsumer

	connected bool
	scMutex   sync.RWMutex
	opts      broker.Options
}

type subscriber struct {
	t    	string
	opts 	broker.SubscribeOptions
	c 	 	rocketmq.PushConsumer
}

type publication struct {
	c			rocketmq.PushConsumer
	m    	*broker.Message
	t    	string
	err  	error
}

func init() {
	cmd.DefaultBrokers["rocketmq"] = NewBroker
}

func (p *publication) Topic() string {
	return p.t
}

func (p *publication) Message() *broker.Message {
	return p.m
}

func (p *publication) Ack() error {
	// TODO p.c
	return nil
}

func (p *publication) Error() error {
	return p.err
}

func (s *subscriber) Options() broker.SubscribeOptions {
	return s.opts
}

func (s *subscriber) Topic() string {
	return s.t
}

func (s *subscriber) Unsubscribe() error {
	return s.c.Shutdown()
}

func (k *rBroker) Address() string {
	if len(k.addrs) > 0 {
		return k.addrs[0]
	}
	return "127.0.0.1:9876"
}

func (k *rBroker) Connect() error {
	if k.isConnected() {
		return nil
	}

	ropts := make([]producer.Option, 0)

	ropts = append(ropts, producer.WithNsResovler(primitive.NewPassthroughResolver(k.opts.Addrs)))

	if retry, ok := k.opts.Context.Value(retryKey{}).(int); ok {
		ropts = append(ropts, producer.WithRetry(retry))
	}

	if credentials, ok := k.opts.Context.Value(credentialsKey{}).(Credentials); ok {
		ropts = append(ropts, producer.WithCredentials(primitive.Credentials{
			AccessKey: credentials.AccessKey,
			SecretKey: credentials.SecretKey,
		}))
	}

	p, err := rocketmq.NewProducer(ropts...)
	if err != nil {
		return err
	}

	err = p.Start()
	if err != nil {
		return err
	}

	k.scMutex.Lock()
	k.p = p
	k.sc = make([]rocketmq.PushConsumer, 0)
	k.connected = true
	k.scMutex.Unlock()

	return nil
}

func (k *rBroker) Disconnect() error {
	if !k.isConnected() {
		return nil
	}
	k.scMutex.Lock()
	defer k.scMutex.Unlock()
	for _, consumer := range k.sc {
		consumer.Shutdown()
	}

	k.sc = nil
	k.p.Shutdown()

	k.connected = false
	return nil
}

func (k *rBroker) Init(opts ...broker.Option) error {
	for _, o := range opts {
		o(&k.opts)
	}
	var cAddrs []string
	for _, addr := range k.opts.Addrs {
		if len(addr) == 0 {
			continue
		}
		cAddrs = append(cAddrs, addr)
	}
	if len(cAddrs) == 0 {
		cAddrs = []string{"127.0.0.1:9876"}
	}
	k.addrs = cAddrs
	return nil
}

func (k *rBroker) isConnected() bool {
	k.scMutex.RLock()
	defer k.scMutex.RUnlock()
	return k.connected
}

func (k *rBroker) Options() broker.Options {
	return k.opts
}

func (k *rBroker) Publish(topic string, msg *broker.Message, opts ...broker.PublishOption) error {
	if !k.isConnected() {
		return errors.New("[rocketmq] broker not connected")
	}

	options := broker.PublishOptions{}
	for _, o := range opts {
		o(&options)
	}

	var (
		delayTimeLevel int
	)

	if options.Context != nil {
		if v, ok := options.Context.Value(delayTimeLevelKey{}).(int); ok {
			delayTimeLevel = v
		}
	}

	m := primitive.NewMessage(topic, []byte(msg.Body))

	for k, v := range msg.Header {
		m.WithProperty(k, v)
	}

	if delayTimeLevel > 0 {
		m.WithDelayTimeLevel(delayTimeLevel)
	}
	_, err := k.p.SendSync(context.Background(), m)
	
	return err
}

func (k *rBroker) getPushConsumer(groupName string) (rocketmq.PushConsumer, error) {
	ropts := make([]consumer.Option, 0)

	ropts = append(ropts, consumer.WithNsResovler(primitive.NewPassthroughResolver(k.opts.Addrs)))
	ropts = append(ropts, consumer.WithConsumerModel(consumer.Clustering))

	if maxReconsumeTimes, ok := k.opts.Context.Value(maxReconsumeTimesKey{}).(int32); ok {
		ropts = append(ropts, consumer.WithMaxReconsumeTimes(maxReconsumeTimes))
	}

	if credentials, ok := k.opts.Context.Value(credentialsKey{}).(Credentials); ok {
		ropts = append(ropts, consumer.WithCredentials(primitive.Credentials{
			AccessKey: credentials.AccessKey,
			SecretKey: credentials.SecretKey,
		}))
	}
	ropts = append(ropts, consumer.WithGroupName(groupName))	

	cs, err := rocketmq.NewPushConsumer(ropts...)
	if err != nil {
		return nil, err
	}

	k.scMutex.Lock()
	defer k.scMutex.Unlock()
	k.sc = append(k.sc, cs)
	return cs, nil
}

func (k *rBroker) Subscribe(topic string, handler broker.Handler, opts ...broker.SubscribeOption) (broker.Subscriber, error) {
	opt := broker.SubscribeOptions{
		AutoAck: true,
		Queue: uuid.New().String(),	// 默认的队列名，会被覆盖的
	}
	for _, o := range opts {
		o(&opt)
	}

	// theoretically, groupName not queue
	// in rocket. one topic have many queue, one queue only belongs to one consumer, one consumer can consume many queue
	// many consumer belongs to a specified group shares messages of the topic 
	groupName := opt.Queue
	if len(groupName) == 0 {
		return nil, errors.New("rocketmq need groupName or queue")
	}

	c, err := k.getPushConsumer(groupName)
	if err != nil {
		return nil, err
	}
	
	err = c.Subscribe(topic, consumer.MessageSelector{}, func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		for _, msg := range msgs {
			header := make(map[string]string)
			for k, v := range msg.GetProperties() {
				header[k] = v
			}
			
			m := &broker.Message{
				Header: header,
				Body: msg.Body,
			}

			p := &publication{c: c, m: m, t: msg.Topic}
			p.err = handler(p)
			if p.err != nil {
				return consumer.ConsumeRetryLater, p.err
			}
		}

		return consumer.ConsumeSuccess, nil
	})
	
	err = c.Start()
	if err != nil {
		return nil, err
	}

	return &subscriber{ t: topic, opts: opt, c: c }, nil
}

func (k *rBroker) String() string {
	return "rocketmq"
}

func NewBroker(opts ...broker.Option) broker.Broker {
	options := broker.Options{
		Context: context.Background(),
	}

	for _, o := range opts {
		o(&options)
	}

	var cAddrs []string
	for _, addr := range options.Addrs {
		if len(addr) == 0 {
			continue
		}
		cAddrs = append(cAddrs, addr)
	}
	if len(cAddrs) == 0 {
		cAddrs = []string{"127.0.0.1:9876"}
	}

	return &rBroker{
		addrs: cAddrs,
		opts:  options,
	}
}