package handlers

import (
	"context"

	"restaurant-booking/internal/amqp"
)

func (a *Handlers) publishEvent(ctx context.Context, routingKey string, payload map[string]any) error {
	return amqp.PublishJSON(ctx, routingKey, payload)
}
