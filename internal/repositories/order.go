package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"my-awesome-project/internal/models"
)

var ErrOrderNotFound = errors.New("order not found")

type OrderRepository struct {
	db *sql.DB
}

func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) Create(ctx context.Context, userID int64, description string) (models.Order, error) {
	var order models.Order
	err := r.db.QueryRowContext(ctx,
		`WITH new_order AS (
		    INSERT INTO orders (user_id, description)
		    VALUES ($1, $2)
		    RETURNING id, user_id, description, status, created_at, updated_at,
		              order_type, plan_duration_months
		)
		SELECT new_order.id, new_order.user_id, users.email, new_order.order_type,
		       new_order.description, COALESCE(new_order.plan_duration_months, 0),
		       new_order.status, new_order.created_at, new_order.updated_at
		FROM new_order
		JOIN users ON users.id = new_order.user_id`,
		userID, description,
	).Scan(&order.ID, &order.UserID, &order.UserEmail, &order.Type, &order.Description,
		&order.PlanDurationMonths, &order.Status, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return models.Order{}, fmt.Errorf("create order: %w", err)
	}

	return order, nil
}

func (r *OrderRepository) CreatePublicationPlan(ctx context.Context, userID int64, durationMonths int) (models.Order, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Order{}, fmt.Errorf("begin publication plan transaction: %w", err)
	}
	defer tx.Rollback()

	var lockedUserID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&lockedUserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Order{}, models.ErrPublicationPlanExists
		}
		return models.Order{}, fmt.Errorf("lock publication plan owner: %w", err)
	}
	var exists bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM orders
			WHERE user_id = $1 AND order_type = 'publication_plan'
			  AND status IN ('pending', 'processing')
		)`,
		userID,
	).Scan(&exists); err != nil {
		return models.Order{}, fmt.Errorf("check existing publication plan: %w", err)
	}
	if exists {
		return models.Order{}, models.ErrPublicationPlanExists
	}

	var order models.Order
	err = tx.QueryRowContext(ctx,
		`WITH new_order AS (
		    INSERT INTO orders (user_id, description, order_type, plan_duration_months)
		    VALUES ($1, $2, 'publication_plan', $3)
		    RETURNING id, user_id, description, status, created_at, updated_at,
		              order_type, plan_duration_months
		)
		SELECT new_order.id, new_order.user_id, users.email, new_order.order_type,
		       new_order.description, new_order.plan_duration_months,
		       new_order.status, new_order.created_at, new_order.updated_at
		FROM new_order
		JOIN users ON users.id = new_order.user_id`,
		userID, fmt.Sprintf("Подписка на публикации на %d мес.", durationMonths), durationMonths,
	).Scan(&order.ID, &order.UserID, &order.UserEmail, &order.Type, &order.Description,
		&order.PlanDurationMonths, &order.Status, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return models.Order{}, fmt.Errorf("create publication plan order: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return models.Order{}, fmt.Errorf("commit publication plan order: %w", err)
	}
	return order, nil
}

func (r *OrderRepository) GetPublicationPlan(ctx context.Context, userID int64) (models.PublicationPlan, error) {
	var plan models.PublicationPlan
	var subscriptionExpiresAt sql.NullTime
	err := r.db.QueryRowContext(ctx,
		`SELECT
		    EXISTS (SELECT 1 FROM orders WHERE user_id = $1 AND order_type = 'publication_plan' AND status = 'completed' AND subscription_expires_at > CURRENT_TIMESTAMP),
		    EXISTS (SELECT 1 FROM orders WHERE user_id = $1 AND order_type = 'publication_plan' AND status IN ('pending', 'processing')),
		    COALESCE((
		        SELECT id FROM orders WHERE user_id = $1 AND order_type = 'publication_plan' AND status IN ('pending', 'processing')
		        ORDER BY id DESC LIMIT 1
		    ), COALESCE((
		        SELECT id FROM orders WHERE user_id = $1 AND order_type = 'publication_plan'
		        ORDER BY id DESC LIMIT 1
		    ), 0)),
		    COALESCE((
		        SELECT plan_duration_months FROM orders WHERE user_id = $1 AND order_type = 'publication_plan'
		        ORDER BY id DESC LIMIT 1
		    ), 0),
		    COALESCE((
		        SELECT status FROM orders WHERE user_id = $1 AND order_type = 'publication_plan'
		        ORDER BY id DESC LIMIT 1
		    ), ''),
		    COALESCE((
		        SELECT updated_at FROM orders WHERE user_id = $1 AND order_type = 'publication_plan'
		        ORDER BY id DESC LIMIT 1
		    ), CURRENT_TIMESTAMP),
		    (
		        SELECT MAX(subscription_expires_at) FROM orders
		        WHERE user_id = $1 AND order_type = 'publication_plan' AND status = 'completed'
		    )`,
		userID,
	).Scan(&plan.Active, &plan.Pending, &plan.OrderID, &plan.DurationMonths, &plan.Status, &plan.UpdatedAt, &subscriptionExpiresAt)
	if err != nil {
		return models.PublicationPlan{}, fmt.Errorf("get publication plan: %w", err)
	}
	if subscriptionExpiresAt.Valid {
		plan.SubscriptionExpiresAt = &subscriptionExpiresAt.Time
	}
	return plan, nil
}

func (r *OrderRepository) ListByUserID(ctx context.Context, userID int64) ([]models.Order, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, description, status, created_at, updated_at
		 FROM orders
		 WHERE user_id = $1
		 ORDER BY created_at DESC, id DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	orders := make([]models.Order, 0)
	for rows.Next() {
		var order models.Order
		if err := rows.Scan(&order.ID, &order.UserID, &order.Description, &order.Status, &order.CreatedAt, &order.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orders: %w", err)
	}

	return orders, nil
}

func (r *OrderRepository) UpdateStatus(ctx context.Context, orderID int64, status string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE orders AS target
		 SET status = $1,
		     updated_at = CURRENT_TIMESTAMP,
		     subscription_expires_at = CASE
		         WHEN target.order_type = 'publication_plan' AND $1 = 'completed' AND target.status <> 'completed'
		         THEN GREATEST(
		             CURRENT_TIMESTAMP,
		             COALESCE((
		                 SELECT MAX(existing.subscription_expires_at)
		                 FROM orders AS existing
		                 WHERE existing.user_id = target.user_id
		                   AND existing.order_type = 'publication_plan'
		                   AND existing.status = 'completed'
		             ), CURRENT_TIMESTAMP)
		         ) + make_interval(months => COALESCE(target.plan_duration_months, 0))
		         ELSE target.subscription_expires_at
		     END
		 WHERE target.id = $2`,
		status, orderID,
	)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check updated order status: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("%w: %d", ErrOrderNotFound, orderID)
	}

	return nil
}
