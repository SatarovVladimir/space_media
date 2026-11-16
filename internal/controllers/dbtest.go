package controllers

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
)

const maxDBTestBodySize = 1 << 20

type dbTestService interface {
	SaveMessage(ctx context.Context, message string) error
}

type DBTestController struct {
	service dbTestService
}

func NewDBTestController(service dbTestService) *DBTestController {
	return &DBTestController{service: service}
}

func (c *DBTestController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/dbtest" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxDBTestBodySize)
	message, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "could not read request body", http.StatusBadRequest)
		return
	}
	if len(message) == 0 {
		http.Error(w, "request body is required", http.StatusBadRequest)
		return
	}

	if err := c.service.SaveMessage(r.Context(), string(message)); err != nil {
		slog.ErrorContext(r.Context(), "save dbtest message", "error", err)
		http.Error(w, "could not save message", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}
