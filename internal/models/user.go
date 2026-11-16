package models

import (
	"errors"
	"time"
)

var ErrEmailAlreadyExists = errors.New("email already exists")

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}
