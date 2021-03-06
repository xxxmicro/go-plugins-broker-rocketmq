package rocketmq_test

import (
	"errors"
	"sync"
	"testing"
	"time"
	"fmt"
	"encoding/json"
	micro "github.com/micro/go-micro/v2"
	broker "github.com/micro/go-micro/v2/broker"
	server "github.com/micro/go-micro/v2/server"
	rocketmq "github.com/xxxmicro/go-plugins-broker-rocketmq/v2"
	"github.com/apache/rocketmq-client-go/v2/rlog"
)

type MyMessage struct {
	ID string	`json:"id"`
	Sender string `json:"sender"`
	Content string `json:"content"`
}

type Example struct{}

func TestPublish(t *testing.T) {
	b := rocketmq.NewBroker(
		broker.Addrs("http://localhost:9876"),
		rocketmq.WithRetry(0),
	)
	b.Init()
	if err := b.Connect(); err != nil {
		t.Logf("cant connect to broker, skip: %v", err)
		t.Skip()
	}

	for i := 1; i <= 1; i++ {
		sender := fmt.Sprintf("sender-%d", i)
		content := fmt.Sprintf("第%d条内容", i)
		
		
		m := MyMessage{ ID: fmt.Sprintf("id-%d", i), Sender: sender, Content: content}
		
		body, _ := json.Marshal(m)
		
		now := time.Now()
		datetime := fmt.Sprintf("%d-%d-%d %d:%d:%d",now.Year(),now.Month(),now.Day(),now.Hour(),now.Minute(),now.Second())

		msg := &broker.Message{
			Header: map[string]string {
				"timestamp": datetime,
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
	rlog.SetLogLevel("info")

	b := rocketmq.NewBroker(
		broker.Addrs("http://localhost:9876"),	// ipaddr: 127.0.0.1:9876, url: http://localhost:9876
		rocketmq.WithMaxReconsumeTimes(3),
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

	var lock sync.Mutex
	count := 0
	_, err := b.Subscribe("messages", func(p broker.Event) error {
		lock.Lock()
		defer lock.Unlock()

		m := p.Message()

		timestamp := m.Header["timestamp"]

		count += 1

		var msg MyMessage
		err := json.Unmarshal(m.Body, &msg)
		if err != nil {
			fmt.Printf("Subscribe err: %v\n", err)
			return err
		}
		fmt.Printf("Subscribe message Topic: %s, ID: %s, ts: %s, count: %d\n", p.Topic(), msg.ID, timestamp, count)
		
		return errors.New("only for test retry")
	}, broker.Queue("default"))

	if err = service.Run(); err != nil {
		t.Fatal(err)
	}
}
