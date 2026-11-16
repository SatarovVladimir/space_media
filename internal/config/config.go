package config

import (
	"fmt"
	"os"
)

type Config struct {
	HTTPPort    string
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	JWTSecret   string
	RabbitMQURL string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPPort:    env("APP_PORT", "8080"),
		DBHost:      env("DB_HOST", "localhost"),
		DBPort:      env("DB_PORT", "5432"),
		DBUser:      env("DB_USER", "space_media_user"),
		DBPassword:  os.Getenv("DB_PASSWORD"),
		DBName:      env("DB_NAME", "space_media"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		RabbitMQURL: env("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
	}

	if cfg.DBPassword == "" {
		return Config{}, fmt.Errorf("DB_PASSWORD environment variable is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET environment variable is required")
	}

	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
