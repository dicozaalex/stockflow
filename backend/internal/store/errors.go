package store

import (
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("not found")

type InsufficientStockError struct {
	SKU       string
	Available int
}

func (e *InsufficientStockError) Error() string {
	return fmt.Sprintf("insufficient stock for %s: available %d", e.SKU, e.Available)
}
