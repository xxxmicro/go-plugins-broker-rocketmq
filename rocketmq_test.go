package rocketmq_test

import (
	"context"
	"os"
	"testing"

	micro "github.com/micro/go-micro/v2"
	broker "github.com/micro/go-micro/v2/broker"
	server "github.com/micro/go-micro/v2/server"
	rabbitmq "github.com/xxxmicro/go-plugins-broker-rocketmq/v2"
)

type Example struct{}

func (e *Example) Handler(ctx context.Context, r interface{}) error {
	return nil
}

func TestOffset(t *testing.T) {
	sub := broker.NewSubscribeOptions(	
	)

	b := rocketmq.NewBroker()
	b.Init()
	if err := b.Connect(); err != nil {
		t.Logf("cant connect to broker, skip: %v", err)
		t.Skip()
	}

	s := server.NewServer(server.Broker(b))

	service := micro.NewService(
		micro.Server(s),
		micro.Broker(b),
	)
	h := &Example{}

	// Register a subscriber
	micro.RegisterSubscriber(
		"topic",
		service.Server(),
		h.Handler,
		server.SubscriberContext(sub.Context),
		server.SubscribeQueue("queue.default"),
	)

	if err := service.Run(); err != nil {
		t.Fatal(err)
	}
}