package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"my-awesome-project/internal/broker"
)

func main() {
	if err := run(); err != nil {
		slog.Error("order worker stopped with an error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	rabbitMQURL := os.Getenv("RABBITMQ_URL")
	if rabbitMQURL == "" {
		rabbitMQURL = "amqp://guest:guest@localhost:5672/"
	}

	rabbitMQ, err := broker.OpenRabbitMQ(rabbitMQURL)
	if err != nil {
		return fmt.Errorf("initialize RabbitMQ: %w", err)
	}
	defer func() {
		if err := rabbitMQ.Close(); err != nil {
			slog.Error("close RabbitMQ connection", "error", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("order worker is waiting for messages")
	if err := NewWorker(rabbitMQ).Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("run order worker: %w", err)
	}

	slog.Info("order worker stopped")
	return nil
}
