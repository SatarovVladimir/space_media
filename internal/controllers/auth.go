package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"my-awesome-project/internal/dto"
	"my-awesome-project/internal/models"
	"my-awesome-project/internal/services"
)

const maxAuthBodySize = 1 << 20

type authService interface {
	Register(ctx context.Context, email, password string) (models.User, string, error)
	Login(ctx context.Context, email, password string) (models.User, string, error)
}

type AuthController struct {
	service authService
}

func NewAuthController(service authService) *AuthController {
	return &AuthController{service: service}
}

func (c *AuthController) Register(w http.ResponseWriter, r *http.Request) {
	var input dto.RegisterRequest
	if err := decodeJSON(w, r, &input); err != nil {
		http.Error(w, "invalid JSON request", http.StatusBadRequest)
		return
	}

	user, token, err := c.service.Register(r.Context(), input.Email, input.Password)
	if err != nil {
		if errors.Is(err, services.ErrInvalidInput) {
			http.Error(w, "valid email and password of 8 to 72 bytes are required", http.StatusBadRequest)
			return
		}
		if errors.Is(err, models.ErrEmailAlreadyExists) {
			http.Error(w, "email is already registered", http.StatusConflict)
			return
		}
		slog.ErrorContext(r.Context(), "register user", "error", err)
		http.Error(w, "could not register user", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, dto.AuthResponse{
		Token: token,
		User:  dto.UserInfo{ID: user.ID, Email: user.Email},
	})
}

func (c *AuthController) Login(w http.ResponseWriter, r *http.Request) {
	var input dto.LoginRequest
	if err := decodeJSON(w, r, &input); err != nil {
		http.Error(w, "invalid JSON request", http.StatusBadRequest)
		return
	}

	user, token, err := c.service.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			http.Error(w, "invalid email or password", http.StatusUnauthorized)
			return
		}
		slog.ErrorContext(r.Context(), "login user", "error", err)
		http.Error(w, "could not log in", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, dto.AuthResponse{
		Token: token,
		User:  dto.UserInfo{ID: user.ID, Email: user.Email},
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodySize)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}

	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("write JSON response", "error", err)
	}
}
