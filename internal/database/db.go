package database

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"

	"my-awesome-project/internal/config"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Open(ctx context.Context, cfg config.Config) (*sql.DB, error) {
	connectionURL := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.DBUser, cfg.DBPassword),
		Host:   net.JoinHostPort(cfg.DBHost, cfg.DBPort),
		Path:   cfg.DBName,
	}

	query := connectionURL.Query()
	query.Set("sslmode", "disable")
	connectionURL.RawQuery = query.Encode()

	db, err := sql.Open("pgx", connectionURL.String())
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL connection: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}

	return db, nil
}
