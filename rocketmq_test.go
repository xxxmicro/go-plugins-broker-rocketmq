package rocketmq_test

import (
	"testing"
	"time"
	"fmt"
	"encoding/json"
	micro "github.com/micro/go-micro/v2"
	broker "github.com/micro/go-micro/v2/broker"
	server "github.com/micro/go-micro/v2/server"
	rocketmq "github.com/xxxmicro/go-plugins-broker-rocketmq/v2"
)

type MyMessage struct {
	ID string	`json:"id"`
	Sender string `json:"sender"`
	Content string `json:"content"`
}

type Example struct{}

func TestPublish(t *testing.T) {
	b := rocketmq.NewBroker(
		broker.Addrs("127.0.0.1:9876"),
	)
	b.Init()
	if err := b.Connect(); err != nil {
		t.Logf("cant connect to broker, skip: %v", err)
		t.Skip()
	}

	for i := 1; i <= 10; i++ {
		sender := fmt.Sprintf("sender-%d", i)
		content := fmt.Sprintf("第%d条内容", i)
		
		
		m := MyMessage{ ID: fmt.Sprintf("%d", i), Sender: sender, Content: content}
		
		body, _ := json.Marshal(m)
		
		msg := &broker.Message{
			Header: map[string]string {
				"timestamp": fmt.Sprintf("%d", time.Now().UnixNano()),
			},
			Body: body,
		}

		err := b.Publish("messages", msg)
		if err != nil {
			t.Logf("publish error: %v", err)
		}
	}	
}


func TestSubscribe(t *testing.T) {
	b := rocketmq.NewBroker(
		broker.Addrs("127.0.0.1:9876"),
	)
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

	_, err := b.Subscribe("messages", func(p broker.Event) error {
		m := p.Message()
		fmt.Printf("Subscribe message: %v\n", m)

		var msg MyMessage
		err := json.Unmarshal(m.Body, &msg)
		if err != nil {
			return err
		}
		fmt.Printf("Subscribe message: %v\n", msg)
		
		return nil	
	})

	if err = service.Run(); err != nil {
		t.Fatal(err)
	}
}