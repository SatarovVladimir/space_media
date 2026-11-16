package dto

import "time"

type CreateOrderRequest struct {
	Description string `json:"description"`
}

type OrderResponse struct {
	ID          int64     `json:"id"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewOrderResponse(id int64, description, status string, createdAt, updatedAt time.Time) OrderResponse {
	return OrderResponse{
		ID:          id,
		Description: description,
		Status:      status,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}
