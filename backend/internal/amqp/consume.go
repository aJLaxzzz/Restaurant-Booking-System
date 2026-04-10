package amqp

import (
	"context"
	"log"
)

// RunConsumer подписывается на exchange events (user.#, reservation.#, payment.#) и вызывает handler для каждого сообщения.
func RunConsumer(ctx context.Context, queue string, handler func(context.Context, string, []byte) error) error {
	mu.Lock()
	chLocal := ch
	mu.Unlock()
	if chLocal == nil {
		return nil
	}
	if _, err := chLocal.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return err
	}
	for _, key := range []string{"user.#", "reservation.#", "payment.#"} {
		if err := chLocal.QueueBind(queue, key, "events", false, nil); err != nil {
			return err
		}
	}
	msgs, err := chLocal.Consume(queue, "", true, false, false, false, nil)
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case d, ok := <-msgs:
				if !ok {
					return
				}
				if err := handler(context.Background(), d.RoutingKey, d.Body); err != nil {
					log.Printf("amqp consumer %s: %v", d.RoutingKey, err)
				}
			}
		}
	}()
	return nil
}
