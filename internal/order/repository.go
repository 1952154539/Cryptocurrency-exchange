package order

import (
	"context"
	"fmt"
	"time"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StoredOrder is the database representation of an order.
type StoredOrder struct {
	ID            string
	OrderID       string
	ClientOrderID string
	UserID        string
	Symbol        string
	Side          string
	Type          string
	TimeInForce   string
	Price         string
	StopPrice     string
	Quantity      string
	FilledQty     string
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	FilledAt      *time.Time
}

// Repository handles order persistence.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates an order repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create inserts a new order into the database.
func (r *Repository) Create(ctx context.Context, order *StoredOrder) error {
	query := `
		INSERT INTO orders (order_id, client_order_id, user_id, symbol, side, type, time_in_force, price, stop_price, quantity, filled_qty, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := r.pool.Exec(ctx, query,
		order.OrderID, order.ClientOrderID, order.UserID, order.Symbol,
		order.Side, order.Type, order.TimeInForce, order.Price, order.StopPrice,
		order.Quantity, order.FilledQty, order.Status,
	)
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}
	return nil
}

// UpdateStatus updates the status and filled quantity of an order.
func (r *Repository) UpdateStatus(ctx context.Context, orderID string, status common.OrderStatus, filledQty decimal.Decimal) error {
	query := `UPDATE orders SET status = $1, filled_qty = $2, updated_at = NOW() WHERE order_id = $3`
	_, err := r.pool.Exec(ctx, query, string(status), filledQty.String(), orderID)
	return err
}

// GetByID retrieves an order by its public order ID.
func (r *Repository) GetByID(ctx context.Context, orderID string) (*StoredOrder, error) {
	query := `
		SELECT id, order_id, client_order_id, user_id, symbol, side, type, time_in_force,
		       price, stop_price, quantity, filled_qty, status, created_at, updated_at, filled_at
		FROM orders WHERE order_id = $1
	`
	row := r.pool.QueryRow(ctx, query, orderID)
	o := &StoredOrder{}
	err := row.Scan(
		&o.ID, &o.OrderID, &o.ClientOrderID, &o.UserID, &o.Symbol,
		&o.Side, &o.Type, &o.TimeInForce, &o.Price, &o.StopPrice,
		&o.Quantity, &o.FilledQty, &o.Status, &o.CreatedAt, &o.UpdatedAt, &o.FilledAt,
	)
	if err == pgx.ErrNoRows {
		return nil, common.ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get order: %w", err)
	}
	return o, nil
}

// GetOpenOrders returns all open orders for a user, optionally filtered by symbol.
func (r *Repository) GetOpenOrders(ctx context.Context, userID string, symbol string) ([]*StoredOrder, error) {
	var rows pgx.Rows
	var err error

	if symbol != "" {
		rows, err = r.pool.Query(ctx,
			`SELECT order_id, client_order_id, user_id, symbol, side, type, time_in_force,
			        price, stop_price, quantity, filled_qty, status, created_at
			 FROM orders WHERE user_id = $1 AND symbol = $2 AND status IN ('open', 'partially_filled')
			 ORDER BY created_at DESC`, userID, symbol)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT order_id, client_order_id, user_id, symbol, side, type, time_in_force,
			        price, stop_price, quantity, filled_qty, status, created_at
			 FROM orders WHERE user_id = $1 AND status IN ('open', 'partially_filled')
			 ORDER BY created_at DESC`, userID)
	}
	if err != nil {
		return nil, fmt.Errorf("get open orders: %w", err)
	}
	defer rows.Close()

	var orders []*StoredOrder
	for rows.Next() {
		o := &StoredOrder{}
		if err := rows.Scan(
			&o.OrderID, &o.ClientOrderID, &o.UserID, &o.Symbol,
			&o.Side, &o.Type, &o.TimeInForce, &o.Price, &o.StopPrice,
			&o.Quantity, &o.FilledQty, &o.Status, &o.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		orders = append(orders, o)
	}
	return orders, nil
}
