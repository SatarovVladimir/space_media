package repositories

import (
	"context"
	"database/sql"
	"fmt"
)

type DBTestRepository struct {
	db *sql.DB
}

func NewDBTestRepository(db *sql.DB) *DBTestRepository {
	return &DBTestRepository{db: db}
}

func (r *DBTestRepository) SaveMessage(ctx context.Context, message string) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO db_test_messages (message) VALUES ($1)", message)
	if err != nil {
		return fmt.Errorf("save dbtest message: %w", err)
	}

	return nil
}
