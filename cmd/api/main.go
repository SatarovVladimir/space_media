package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"my-awesome-project/internal/broker"
	"my-awesome-project/internal/config"
	"my-awesome-project/internal/controllers"
	"my-awesome-project/internal/database"
	"my-awesome-project/internal/metrics"
	"my-awesome-project/internal/middleware"
	"my-awesome-project/internal/migrations"
	"my-awesome-project/internal/repositories"
	"my-awesome-project/internal/routes"
	"my-awesome-project/internal/services"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped with an error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	jwtManager, err := middleware.NewJWTManager(cfg.JWTSecret)
	if err != nil {
		return fmt.Errorf("initialize JWT: %w", err)
	}

	connectContext, cancelConnect := context.WithTimeout(context.Background(), 5*time.Second)
	db, err := database.Open(connectContext, cfg)
	cancelConnect()
	if err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("close database connection", "error", err)
		}
	}()
	slog.Info("connected to PostgreSQL", "database", cfg.DBName)

	rabbitMQ, err := broker.OpenRabbitMQ(cfg.RabbitMQURL)
	if err != nil {
		return fmt.Errorf("initialize RabbitMQ: %w", err)
	}
	defer func() {
		if err := rabbitMQ.Close(); err != nil {
			slog.Error("close RabbitMQ connection", "error", err)
		}
	}()
	slog.Info("connected to RabbitMQ")

	migrationContext, cancelMigration := context.WithTimeout(context.Background(), 5*time.Second)
	if err := migrations.Apply(migrationContext, db); err != nil {
		cancelMigration()
		return fmt.Errorf("initialize database schema: %w", err)
	}
	cancelMigration()
	slog.Info("database schema is ready")

	testRepository := repositories.NewTestRepository()
	testService := services.NewTestService(testRepository)
	testController := controllers.NewTestController(testService)
	dbTestRepository := repositories.NewDBTestRepository(db)
	dbTestService := services.NewDBTestService(dbTestRepository)
	dbTestController := controllers.NewDBTestController(dbTestService)
	userRepository := repositories.NewUserRepository(db)
	authService := services.NewAuthService(userRepository, jwtManager)
	authController := controllers.NewAuthController(authService)
	orderRepository := repositories.NewOrderRepository(db)
	orderService := services.NewOrderService(orderRepository, rabbitMQ)
	ordersController := controllers.NewOrdersController(orderService)
	socialRepository := repositories.NewSocialRepository(db)
	socialService := services.NewSocialService(socialRepository)
	avatarDir := os.Getenv("AVATAR_DIR")
	if avatarDir == "" {
		avatarDir = "uploads/avatars"
	}
	socialController := controllers.NewSocialController(socialService, orderService, avatarDir)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	statusConsumerErrors := make(chan error, 1)
	go func() {
		err := rabbitMQ.ConsumeOrderStatuses(ctx, func(ctx context.Context, event broker.OrderStatusChanged) error {
			if err := orderRepository.UpdateStatus(ctx, event.OrderID, event.Status); err != nil {
				if errors.Is(err, repositories.ErrOrderNotFound) {
					slog.WarnContext(ctx, "status event references a missing order", "order_id", event.OrderID)
					return nil
				}
				return fmt.Errorf("save order status: %w", err)
			}
			slog.InfoContext(ctx, "order status updated", "order_id", event.OrderID, "status", event.Status)
			return nil
		})
		if err != nil {
			statusConsumerErrors <- fmt.Errorf("consume order status events: %w", err)
		}
	}()

	httpMetrics := metrics.NewHTTP()
	apiHandler := httpMetrics.Instrument(routes.New(testController, dbTestController,
		http.HandlerFunc(authController.Register),
		http.HandlerFunc(authController.Login),
		ordersController,
		socialController,
		jwtManager.Authenticate,
		jwtManager.AuthenticateOptional))
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", httpMetrics.Handler())
	mux.Handle("GET /uploads/avatars/", http.StripPrefix("/uploads/avatars/",
		http.FileServer(http.Dir(avatarDir))))
	mux.Handle("/", apiHandler)

	server := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("HTTP server is listening", "address", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	var runErr error
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case err := <-statusConsumerErrors:
		runErr = err
		stop()
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("gracefully shut down HTTP server: %w", err)
	}

	if err := <-serverErrors; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP during shutdown: %w", err)
	}

	slog.Info("HTTP server stopped")
	return runErr
}
