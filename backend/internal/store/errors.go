package store

import (
	"errors"
	"fmt"
	"strings"
)

var ErrNotFound = errors.New("not found")

type InsufficientStockError struct {
	SKU       string
	Available int
}

func (e *InsufficientStockError) Error() string {
	return fmt.Sprintf("insufficient stock for %s: available %d", e.SKU, e.Available)
}

type UnknownSKUError struct {
	SKUs []string
}

func (e *UnknownSKUError) Error() string {
	return "unknown sku: " + strings.Join(e.SKUs, ", ")
}

type InactiveProductError struct {
	SKUs []string
}

func (e *InactiveProductError) Error() string {
	return "inactive product: " + strings.Join(e.SKUs, ", ")
}
