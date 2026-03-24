package amqp

import (
	"context"
	"encoding/json"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	mu     sync.Mutex
	conn   *amqp.Connection
	ch     *amqp.Channel
	urlStr string
)

func Init(url string) error {
	if url == "" {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	urlStr = url
	c, err := amqp.Dial(url)
	if err != nil {
		return err
	}
	chl, err := c.Channel()
	if err != nil {
		_ = c.Close()
		return err
	}
	_ = chl.ExchangeDeclare("events", "topic", true, false, false, false, nil)
	conn = c
	ch = chl
	return nil
}

func Close() {
	mu.Lock()
	defer mu.Unlock()
	if ch != nil {
		_ = ch.Close()
	}
	if conn != nil {
		_ = conn.Close()
	}
}

// PublishJSON шлёт в exchange events с routing key.
func PublishJSON(ctx context.Context, routingKey string, v any) error {
	if ch == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return ch.PublishWithContext(ctx, "events", routingKey, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        b,
	})
}
