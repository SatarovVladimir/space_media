package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const OrdersCreatedQueue = "orders.created"
const OrderStatusQueue = "orders.status"

const (
	OrderStatusProcessing = "processing"
	OrderStatusCompleted  = "completed"
	OrderStatusFailed     = "failed"
)

var errInvalidMessage = errors.New("invalid RabbitMQ message")

type OrderCreated struct {
	OrderID        int64  `json:"order_id"`
	UserID         int64  `json:"user_id"`
	UserEmail      string `json:"user_email,omitempty"`
	OrderType      string `json:"order_type,omitempty"`
	Description    string `json:"description,omitempty"`
	DurationMonths int    `json:"duration_months,omitempty"`
}

type OrderStatusChanged struct {
	OrderID int64  `json:"order_id"`
	Status  string `json:"status"`
}

type RabbitMQ struct {
	connection    *amqp.Connection
	channel       *amqp.Channel
	confirmations <-chan amqp.Confirmation
	publishMu     sync.Mutex
}

func OpenRabbitMQ(url string) (*RabbitMQ, error) {
	connection, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}

	channel, err := connection.Channel()
	if err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("open RabbitMQ channel: %w", err)
	}

	if _, err := channel.QueueDeclare(OrdersCreatedQueue, true, false, false, false, nil); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return nil, fmt.Errorf("declare orders queue: %w", err)
	}
	if err := channel.Confirm(false); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return nil, fmt.Errorf("enable RabbitMQ publisher confirms: %w", err)
	}

	return &RabbitMQ{
		connection:    connection,
		channel:       channel,
		confirmations: channel.NotifyPublish(make(chan amqp.Confirmation, 1)),
	}, nil
}

func (r *RabbitMQ) PublishOrderCreated(ctx context.Context, event OrderCreated) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode order event: %w", err)
	}

	r.publishMu.Lock()
	defer r.publishMu.Unlock()

	if err := r.channel.PublishWithContext(ctx, "", OrdersCreatedQueue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	}); err != nil {
		return fmt.Errorf("publish order event: %w", err)
	}

	select {
	case confirmation, ok := <-r.confirmations:
		if !ok {
			return errors.New("RabbitMQ publisher confirmation channel closed")
		}
		if !confirmation.Ack {
			return errors.New("RabbitMQ did not confirm order event")
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for RabbitMQ confirmation: %w", ctx.Err())
	}
}

func (r *RabbitMQ) PublishOrderStatus(ctx context.Context, event OrderStatusChanged) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode order status event: %w", err)
	}

	r.publishMu.Lock()
	defer r.publishMu.Unlock()

	if _, err := r.channel.QueueDeclare(OrderStatusQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare order status queue: %w", err)
	}
	if err := r.channel.PublishWithContext(ctx, "", OrderStatusQueue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	}); err != nil {
		return fmt.Errorf("publish order status: %w", err)
	}

	select {
	case confirmation, ok := <-r.confirmations:
		if !ok {
			return errors.New("RabbitMQ publisher confirmation channel closed")
		}
		if !confirmation.Ack {
			return errors.New("RabbitMQ did not confirm order status")
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for RabbitMQ status confirmation: %w", ctx.Err())
	}
}

func (r *RabbitMQ) ConsumeOrderStatuses(ctx context.Context, handler func(context.Context, OrderStatusChanged) error) error {
	return r.consume(ctx, OrderStatusQueue, func(ctx context.Context, body []byte) error {
		var event OrderStatusChanged
		if err := json.Unmarshal(body, &event); err != nil {
			return fmt.Errorf("%w: decode order status event: %v", errInvalidMessage, err)
		}
		if event.OrderID <= 0 || (event.Status != OrderStatusProcessing &&
			event.Status != OrderStatusCompleted && event.Status != OrderStatusFailed) {
			return fmt.Errorf("%w: invalid order status event", errInvalidMessage)
		}
		return handler(ctx, event)
	})
}

func (r *RabbitMQ) ConsumeOrders(ctx context.Context, handler func(context.Context, OrderCreated) error) error {
	return r.consume(ctx, OrdersCreatedQueue, func(ctx context.Context, body []byte) error {
		var event OrderCreated
		if err := json.Unmarshal(body, &event); err != nil {
			return fmt.Errorf("%w: decode order event: %v", errInvalidMessage, err)
		}
		if event.OrderID <= 0 || event.UserID <= 0 {
			return fmt.Errorf("%w: invalid order event", errInvalidMessage)
		}
		return handler(ctx, event)
	})
}

func (r *RabbitMQ) consume(ctx context.Context, queue string, handler func(context.Context, []byte) error) error {
	channel, err := r.connection.Channel()
	if err != nil {
		return fmt.Errorf("open %s consumer channel: %w", queue, err)
	}
	defer channel.Close()

	if _, err := channel.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare %s queue: %w", queue, err)
	}
	if err := channel.Qos(1, 0, false); err != nil {
		return fmt.Errorf("set %s consumer prefetch: %w", queue, err)
	}
	deliveries, err := channel.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume %s: %w", queue, err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("RabbitMQ %s consumer channel closed", queue)
			}

			if err := handler(ctx, delivery.Body); err != nil {
				if !errors.Is(err, errInvalidMessage) {
					if nackErr := delivery.Nack(false, true); nackErr != nil {
						return fmt.Errorf("requeue message from %s after handler error %v: %w", queue, err, nackErr)
					}
					slog.Error("message processing failed; will retry",
						"queue", queue, "error", err)
					timer := time.NewTimer(time.Second)
					select {
					case <-ctx.Done():
						timer.Stop()
						return nil
					case <-timer.C:
					}
					continue
				}
				slog.Warn("discarding invalid RabbitMQ message", "queue", queue, "error", err)
				if rejectErr := delivery.Reject(false); rejectErr != nil {
					return fmt.Errorf("reject invalid message from %s: %w", queue, rejectErr)
				}
				continue
			}
			if err := delivery.Ack(false); err != nil {
				return fmt.Errorf("acknowledge message from %s: %w", queue, err)
			}
		}
	}
}

func (r *RabbitMQ) Close() error {
	channelErr := r.channel.Close()
	connectionErr := r.connection.Close()
	if channelErr != nil || connectionErr != nil {
		return errors.Join(channelErr, connectionErr)
	}
	return nil
}
