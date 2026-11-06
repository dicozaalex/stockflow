package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

type OrderItemInput struct {
	SKU string `json:"sku"`
	Qty int    `json:"qty"`
}

type OrderItem struct {
	ProductID int    `json:"-"`
	SKU       string `json:"sku"`
	Qty       int    `json:"qty"`
	UnitPrice int    `json:"unit_price"`
}

type Order struct {
	ID          int         `json:"id"`
	Customer    string      `json:"customer"`
	Status      string      `json:"status"`
	Total       int         `json:"total"`
	CreatedAt   time.Time   `json:"created_at"`
	CancelledAt *time.Time  `json:"cancelled_at"`
	Items       []OrderItem `json:"items"`
}

func (s *Store) CreateOrder(ctx context.Context, customer string, items []OrderItemInput) (*Order, error) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].SKU < items[j].SKU
	})

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	skus := make([]string, len(items))
	for i, it := range items {
		skus[i] = it.SKU
	}
	if err := checkSKUs(ctx, tx, skus); err != nil {
		return nil, err
	}

	orderItems := make([]OrderItem, 0, len(items))
	total := 0

	for _, it := range items {
		var productID, price int

		err := tx.QueryRow(ctx, `
		UPDATE products
		SET stock = stock - $2
		WHERE sku = $1 AND stock >= $2
		RETURNING id, price
		`, it.SKU, it.Qty).Scan(&productID, &price)

		if errors.Is(err, pgx.ErrNoRows) {
			var available int
			err := tx.QueryRow(ctx, `
			SELECT stock FROM products WHERE sku = $1
			`, it.SKU).Scan(&available)

			if err != nil {
				return nil, err
			}
			return nil, &InsufficientStockError{SKU: it.SKU, Available: available}
		}
		if err != nil {
			return nil, err
		}

		total += price * it.Qty
		orderItems = append(orderItems, OrderItem{
			ProductID: productID,
			SKU:       it.SKU,
			Qty:       it.Qty,
			UnitPrice: price,
		})
	}

	order := &Order{
		Customer: customer,
		Total:    total,
		Items:    orderItems,
	}
	err = tx.QueryRow(ctx, `
	INSERT INTO sales_orders (customer, total)
	VALUES ($1, $2)
	RETURNING id, status, created_at
	`, customer, total).Scan(&order.ID, &order.Status, &order.CreatedAt)
	if err != nil {
		return nil, err
	}

	note := fmt.Sprintf("order #%d", order.ID)

	for _, it := range orderItems {
		_, err := tx.Exec(ctx, `
		INSERT INTO sales_order_items (order_id, sku, qty, unit_price)
		VALUES ($1, $2, $3, $4)`,
			order.ID, it.SKU, it.Qty, it.UnitPrice)
		if err != nil {
			return nil, err
		}

		_, err = tx.Exec(ctx, `
		INSERT INTO stock_movements (product_id, type, qty, note)
		VALUES ($1, 'out', $2, $3)`,
			it.ProductID, it.Qty, note,
		)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return order, nil
}

func checkSKUs(ctx context.Context, tx pgx.Tx, skus []string) error {
	rows, err := tx.Query(ctx,
		`SELECT sku, is_active FROM products WHERE sku = ANY($1)`,
		skus)
	if err != nil {
		return err
	}
	defer rows.Close()

	active := make(map[string]bool)
	for rows.Next() {
		var sku string
		var isActive bool
		if err := rows.Scan(&sku, &isActive); err != nil {
			return err
		}
		active[sku] = isActive
	}
	if err := rows.Err(); err != nil {
		return err
	}

	var unknown, inactive []string
	for _, sku := range skus {
		isActive, found := active[sku]
		if !found {
			unknown = append(unknown, sku)
		} else if !isActive {
			inactive = append(inactive, sku)
		}
	}

	if len(unknown) > 0 {
		return &UnknownSKUError{SKUs: unknown}
	}
	if len(inactive) > 0 {
		return &InactiveProductError{SKUs: inactive}
	}
	return nil
}
