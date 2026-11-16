package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"my-awesome-project/internal/models"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

const (
	minPasswordLength = 8
	maxPasswordLength = 72
)

type UserRepository interface {
	Create(ctx context.Context, email, passwordHash string) (models.User, error)
	FindByEmail(ctx context.Context, email string) (models.User, error)
}

type TokenIssuer interface {
	Generate(userID int64) (string, error)
}

type AuthService struct {
	users  UserRepository
	tokens TokenIssuer
}

func NewAuthService(users UserRepository, tokens TokenIssuer) *AuthService {
	return &AuthService{users: users, tokens: tokens}
}

func (s *AuthService) Register(ctx context.Context, email, password string) (models.User, string, error) {
	email, err := normalizeEmail(email)
	if err != nil || len(password) < minPasswordLength || len(password) > maxPasswordLength {
		return models.User{}, "", ErrInvalidInput
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return models.User{}, "", fmt.Errorf("hash password: %w", err)
	}

	user, err := s.users.Create(ctx, email, string(passwordHash))
	if err != nil {
		return models.User{}, "", err
	}

	token, err := s.tokens.Generate(user.ID)
	if err != nil {
		return models.User{}, "", fmt.Errorf("generate session token: %w", err)
	}

	return user, token, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (models.User, string, error) {
	email, err := normalizeEmail(email)
	if err != nil || password == "" {
		return models.User{}, "", ErrInvalidCredentials
	}

	user, err := s.users.FindByEmail(ctx, email)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, "", ErrInvalidCredentials
	}
	if err != nil {
		return models.User{}, "", fmt.Errorf("load user: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return models.User{}, "", ErrInvalidCredentials
	}

	token, err := s.tokens.Generate(user.ID)
	if err != nil {
		return models.User{}, "", fmt.Errorf("generate session token: %w", err)
	}

	return user, token, nil
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) > 254 {
		return "", ErrInvalidInput
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return "", ErrInvalidInput
	}

	return email, nil
}
