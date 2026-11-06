package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://stockflowuser:password123@localhost:5433/stockflowdb"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("cannot reach database (is it running?): %v", err)
	}
	t.Cleanup(pool.Close)

	return New(pool)
}

func createTestProduct(t *testing.T, s *Store, stock int) (int, string) {
	t.Helper()

	sku := fmt.Sprintf("TEST-%d", time.Now().UnixNano())
	var id int
	err := s.pool.QueryRow(context.Background(), `
		INSERT INTO products (sku, name, price, stock)
		VALUES ($1, 'Test Product', 1000, $2)
		RETURNING id`,
		sku, stock,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create test product: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()

		if _, err := s.pool.Exec(ctx,
			`DELETE FROM stock_movements WHERE product_id = $1`,
			id,
		); err != nil {
			t.Errorf("cleanup movements: %v", err)
		}

		if _, err := s.pool.Exec(ctx, `
			WITH deleted AS (
				DELETE FROM sales_order_items WHERE sku = $1 RETURNING order_id
			)
			DELETE FROM sales_orders WHERE id IN (SELECT order_id FROM deleted)`,
			sku,
		); err != nil {
			t.Errorf("cleanup orders: %v", err)
		}

		if _, err := s.pool.Exec(ctx,
			`DELETE FROM products WHERE id = $1`,
			id,
		); err != nil {
			t.Errorf("cleanup product: %v", err)
		}
	})

	return id, sku
}

func queryInt(t *testing.T, s *Store, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := s.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("query %q: %v", sql, err)
	}
	return n
}

func productStock(t *testing.T, s *Store, productID int) int {
	t.Helper()
	return queryInt(t, s, `SELECT stock FROM products WHERE id = $1`, productID)
}

func movementCount(t *testing.T, s *Store, productID int, moveType string) int {
	t.Helper()
	return queryInt(t, s,
		`SELECT COUNT(*) FROM stock_movements WHERE product_id = $1 AND type = $2`,
		productID, moveType)
}

func orderItemCount(t *testing.T, s *Store, sku string) int {
	t.Helper()
	return queryInt(t, s, `SELECT COUNT(*) FROM sales_order_items WHERE sku = $1`, sku)
}

// test a: insufficient stock rejected with NOTHING written
func TestCreateOrder_InsufficientStock_WritesNothing(t *testing.T) {
	s := newTestStore(t)
	id, sku := createTestProduct(t, s, 5)

	_, err := s.CreateOrder(context.Background(), "Test Customer",
		[]OrderItemInput{{SKU: sku, Qty: 10}})

	var insuff *InsufficientStockError
	if !errors.As(err, &insuff) {
		t.Fatalf("expected InsufficientStockError, got %v", err)
	}
	if insuff.Available != 5 {
		t.Fatalf("expected available 5, got %d", insuff.Available)
	}

	if got := productStock(t, s, id); got != 5 {
		t.Fatalf("stock changed: expected 5, got %d", got)
	}
	if got := movementCount(t, s, id, "out"); got != 0 {
		t.Fatalf("expected 0 out movements, got %d", got)
	}
	if got := orderItemCount(t, s, sku); got != 0 {
		t.Fatalf("expected 0 order items, got %d", got)
	}
}

// test b: happy path writes movement + updates stock
func TestCreateOrder_HappyPath(t *testing.T) {
	s := newTestStore(t)
	id, sku := createTestProduct(t, s, 5)

	order, err := s.CreateOrder(context.Background(), "Test Customer",
		[]OrderItemInput{{SKU: sku, Qty: 2}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := productStock(t, s, id); got != 3 {
		t.Fatalf("expected stock 3, got %d", got)
	}
	if got := movementCount(t, s, id, "out"); got != 1 {
		t.Fatalf("expected 1 out movement, got %d", got)
	}
	if got := orderItemCount(t, s, sku); got != 1 {
		t.Fatalf("expected 1 order item, got %d", got)
	}

	if order.Total != 2000 {
		t.Fatalf("expected total 2000, got %d", order.Total)
	}
	if len(order.Items) != 1 {
		t.Fatalf("expected 1 item in order, got %d", len(order.Items))
	}
	if order.Items[0].UnitPrice != 1000 {
		t.Fatalf("expected unit price 1000, got %d", order.Items[0].UnitPrice)
	}
}
