package controllers

import (
	"context"
	"log/slog"
	"net/http"
)

type testService interface {
	Message(ctx context.Context) (string, error)
}

type TestController struct {
	service testService
}

func NewTestController(service testService) *TestController {
	return &TestController{service: service}
}

func (c *TestController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/test" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	message, err := c.service.Message(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "get test message", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(message)); err != nil {
		slog.ErrorContext(r.Context(), "write test response", "error", err)
	}
}
