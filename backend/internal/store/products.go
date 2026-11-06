package store

import (
	"context"
	"time"
)

type Product struct {
	ID             int        `json:"id"`
	SKU            string     `json:"sku"`
	Name           string     `json:"name"`
	Price          int        `json:"price"`
	Stock          int        `json:"stock"`
	IsActive       bool       `json:"is_active"`
	CreatedAt      time.Time  `json:"created_at"`
	LastMovementAt *time.Time `json:"last_movement_at"`
}

func (s *Store) ListProducts(ctx context.Context, search string, limit, offset int) ([]Product, int, error) {
	pattern := "%" + search + "%"

	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.sku, p.name, p.price, p.stock, p.is_active, p.created_at, (SELECT MAX(m.created_at)
		FROM stock_movements m
		WHERE m.product_id = p.id) AS last_movement_at
		FROM products p
		WHERE p.sku ILIKE $1 OR p.name ILIKE $1
		ORDER BY p.id
		LIMIT $2 OFFSET $3
		`, pattern, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	products := make([]Product, 0)
	for rows.Next() {
		var p Product
		err := rows.Scan(
			&p.ID, &p.SKU, &p.Name, &p.Price, &p.Stock,
			&p.IsActive, &p.CreatedAt, &p.LastMovementAt,
		)
		if err != nil {
			return nil, 0, err
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	err = s.pool.QueryRow(ctx, `
		SELECT COUNT (*) FROM products WHERE sku ILIKE $1 OR name ILIKE $1
	`, pattern).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	return products, total, nil
}
