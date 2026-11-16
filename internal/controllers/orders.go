package controllers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"my-awesome-project/internal/dto"
	"my-awesome-project/internal/middleware"
	"my-awesome-project/internal/models"
	"my-awesome-project/internal/services"
)

type orderService interface {
	Create(ctx context.Context, userID int64, description string) (models.Order, error)
	ListByUserID(ctx context.Context, userID int64) ([]models.Order, error)
}

type OrdersController struct {
	service orderService
}

func NewOrdersController(service orderService) *OrdersController {
	return &OrdersController{service: service}
}

func (c *OrdersController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/orders" {
		http.NotFound(w, r)
		return
	}

	userID, ok := middleware.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodPost:
		c.create(w, r, userID)
	case http.MethodGet:
		c.list(w, r, userID)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (c *OrdersController) create(w http.ResponseWriter, r *http.Request, userID int64) {
	var input dto.CreateOrderRequest
	if err := decodeJSON(w, r, &input); err != nil {
		http.Error(w, "invalid JSON request", http.StatusBadRequest)
		return
	}

	order, err := c.service.Create(r.Context(), userID, input.Description)
	if err != nil {
		if errors.Is(err, services.ErrInvalidOrder) {
			http.Error(w, "description must contain 1 to 1000 bytes", http.StatusBadRequest)
			return
		}
		if errors.Is(err, services.ErrOrderDispatch) {
			slog.ErrorContext(r.Context(), "dispatch order for processing", "error", err)
			http.Error(w, "order was saved but could not be sent for processing; check orders list before retrying",
				http.StatusServiceUnavailable)
			return
		}
		slog.ErrorContext(r.Context(), "create order", "error", err)
		http.Error(w, "could not create order", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, dto.NewOrderResponse(
		order.ID, order.Description, order.Status, order.CreatedAt, order.UpdatedAt,
	))
}

func (c *OrdersController) list(w http.ResponseWriter, r *http.Request, userID int64) {
	orders, err := c.service.ListByUserID(r.Context(), userID)
	if err != nil {
		slog.ErrorContext(r.Context(), "list orders", "error", err)
		http.Error(w, "could not list orders", http.StatusInternalServerError)
		return
	}

	response := make([]dto.OrderResponse, 0, len(orders))
	for _, order := range orders {
		response = append(response, dto.NewOrderResponse(
			order.ID, order.Description, order.Status, order.CreatedAt, order.UpdatedAt,
		))
	}
	writeJSON(w, http.StatusOK, response)
}
