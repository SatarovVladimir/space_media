package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"my-awesome-project/internal/models"

	"github.com/jackc/pgx/v5/pgconn"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, email, passwordHash string) (models.User, error) {
	var user models.User
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash, display_name)
		 VALUES ($1, $2, left(split_part($1, '@', 1), 60))
		 RETURNING id, email, password_hash, created_at`,
		email, passwordHash,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return models.User{}, models.ErrEmailAlreadyExists
		}
		return models.User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (models.User, error) {
	var user models.User
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, created_at
		 FROM users
		 WHERE email = $1`,
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return models.User{}, fmt.Errorf("find user by email: %w", err)
	}

	return user, nil
}
