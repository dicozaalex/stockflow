package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type Movement struct {
	ID        int       `json:"id"`
	ProductID int       `json:"product_id"`
	Type      string    `json:"type"`
	Qty       int       `json:"qty"`
	Note      *string   `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) AdjustStock(ctx context.Context, productID int, moveType string, qty int, note string) (int, error) {
	delta := qty
	if moveType == "out" {
		delta = -qty
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var newStock int
	err = tx.QueryRow(ctx, `
	UPDATE products
	SET stock = stock + $2
	WHERE id = $1 AND stock + $2 >= 0
	RETURNING stock
	`, productID, delta).Scan(&newStock)

	if errors.Is(err, pgx.ErrNoRows) {
		var sku string
		var current int
		err = tx.QueryRow(ctx, `
		SELECT sku, stock FROM products WHERE id = $1
		`, productID).Scan(&sku, &current)

		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNotFound
		}
		if err != nil {
			return 0, err
		}
		return 0, &InsufficientStockError{SKU: sku, Available: current}
	}
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(ctx, `
	INSERT INTO stock_movements (product_id, type, qty, note)
	VALUES ($1, $2, $3, NULLIF($4, ''))
	`, productID, moveType, qty, note)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return newStock, nil
}

func (s *Store) ListMovements(ctx context.Context, productID int) ([]Movement, error) {
	rows, err := s.pool.Query(ctx, `
	SELECT id, product_id, type, qty, note, created_at
	FROM stock_movements
	WHERE product_id = $1
	ORDER BY created_at DESC, id DESC
	`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	movements := make([]Movement, 0)
	for rows.Next() {
		var m Movement
		if err := rows.Scan(&m.ID, &m.ProductID, &m.Type, &m.Qty, &m.Note, &m.CreatedAt); err != nil {
			return nil, err
		}
		movements = append(movements, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return movements, nil
}
