package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"my-awesome-project/internal/broker"
	"my-awesome-project/internal/models"
)

var (
	ErrInvalidOrder           = errors.New("invalid order")
	ErrOrderDispatch          = errors.New("order created but could not be sent for processing")
	ErrPublicationPlanPending = errors.New("publication plan order is already pending")
	ErrInvalidPlanDuration    = errors.New("publication plan duration must be 1 or 12 months")
)

const maxOrderDescriptionLength = 1000

type OrderRepository interface {
	Create(ctx context.Context, userID int64, description string) (models.Order, error)
	ListByUserID(ctx context.Context, userID int64) ([]models.Order, error)
	CreatePublicationPlan(ctx context.Context, userID int64, durationMonths int) (models.Order, error)
	GetPublicationPlan(ctx context.Context, userID int64) (models.PublicationPlan, error)
}

type OrderPublisher interface {
	PublishOrderCreated(ctx context.Context, event broker.OrderCreated) error
}

type OrderService struct {
	repository OrderRepository
	publisher  OrderPublisher
}

func NewOrderService(repository OrderRepository, publisher OrderPublisher) *OrderService {
	return &OrderService{repository: repository, publisher: publisher}
}

func (s *OrderService) Create(ctx context.Context, userID int64, description string) (models.Order, error) {
	description = strings.TrimSpace(description)
	if userID <= 0 || description == "" || !utf8.ValidString(description) || len(description) > maxOrderDescriptionLength {
		return models.Order{}, ErrInvalidOrder
	}
	order, err := s.repository.Create(ctx, userID, description)
	if err != nil {
		return models.Order{}, err
	}
	return s.dispatch(ctx, order)
}

func (s *OrderService) CreatePublicationPlan(ctx context.Context, userID int64, durationMonths int) (models.Order, error) {
	if userID <= 0 {
		return models.Order{}, ErrInvalidOrder
	}
	if durationMonths != 1 && durationMonths != 12 {
		return models.Order{}, ErrInvalidPlanDuration
	}
	order, err := s.repository.CreatePublicationPlan(ctx, userID, durationMonths)
	if errors.Is(err, models.ErrPublicationPlanExists) {
		return models.Order{}, ErrPublicationPlanPending
	}
	if err != nil {
		return models.Order{}, err
	}
	return s.dispatch(ctx, order)
}

func (s *OrderService) PublicationPlan(ctx context.Context, userID int64) (models.PublicationPlan, error) {
	if userID <= 0 {
		return models.PublicationPlan{}, ErrInvalidOrder
	}
	return s.repository.GetPublicationPlan(ctx, userID)
}

func (s *OrderService) dispatch(ctx context.Context, order models.Order) (models.Order, error) {
	if err := s.publisher.PublishOrderCreated(ctx, broker.OrderCreated{
		OrderID:        order.ID,
		UserID:         order.UserID,
		UserEmail:      order.UserEmail,
		OrderType:      order.Type,
		Description:    order.Description,
		DurationMonths: order.PlanDurationMonths,
	}); err != nil {
		return models.Order{}, fmt.Errorf("%w: %v", ErrOrderDispatch, err)
	}

	return order, nil
}

func (s *OrderService) ListByUserID(ctx context.Context, userID int64) ([]models.Order, error) {
	if userID <= 0 {
		return nil, ErrInvalidOrder
	}

	return s.repository.ListByUserID(ctx, userID)
}
