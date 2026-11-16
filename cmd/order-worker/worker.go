package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"my-awesome-project/internal/broker"
)

type messageBroker interface {
	ConsumeOrders(ctx context.Context, handler func(context.Context, broker.OrderCreated) error) error
	PublishOrderStatus(ctx context.Context, event broker.OrderStatusChanged) error
}

type Worker struct {
	broker messageBroker
	delay  time.Duration
}

func NewWorker(broker messageBroker) *Worker {
	return &Worker{broker: broker, delay: 15 * time.Second}
}

func (w *Worker) Run(ctx context.Context) error {
	return w.broker.ConsumeOrders(ctx, w.process)
}

func (w *Worker) process(ctx context.Context, event broker.OrderCreated) error {
	slog.InfoContext(ctx, "processing order",
		"order_id", event.OrderID,
		"user_id", event.UserID,
		"order_type", event.OrderType,
		"duration_months", event.DurationMonths,
	)
	if err := w.broker.PublishOrderStatus(ctx, broker.OrderStatusChanged{
		OrderID: event.OrderID,
		Status:  broker.OrderStatusProcessing,
	}); err != nil {
		return fmt.Errorf("publish processing status: %w", err)
	}

	timer := time.NewTimer(w.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}

	if err := w.broker.PublishOrderStatus(ctx, broker.OrderStatusChanged{
		OrderID: event.OrderID,
		Status:  broker.OrderStatusCompleted,
	}); err != nil {
		return fmt.Errorf("publish completed status: %w", err)
	}

	slog.InfoContext(ctx, "order processing completed", "order_id", event.OrderID)
	return nil
}
