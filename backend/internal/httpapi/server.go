package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/dicozaalex/stockflow/backend/internal/store"
)

type Server struct {
	store *store.Store
}

func NewServer(st *store.Store) *Server {
	return &Server{store: st}
}

type adjustStockRequest struct {
	Type string `json:"type"`
	Qty  int    `json:"qty"`
	Note string `json:"note"`
}

type createOrderRequest struct {
	Customer string                 `json:"customer"`
	Items    []store.OrderItemInput `json:"items"`
}

// routes
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/products", s.listProducts)
	mux.HandleFunc("POST /api/products/{id}/stock", s.adjustStock)
	mux.HandleFunc("GET /api/products/{id}/movements", s.listMovements)
	mux.HandleFunc("POST /api/orders", s.createOrder)
	return mux
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		WriteError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "Database is unreachable", nil)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})

}

func (s *Server) listProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	search := q.Get("search")

	page, err := intParam(q.Get("page"), 1)
	if err != nil || page < 1 {
		WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "page must be an integer >= 1", nil)
		return
	}
	limit, err := intParam(q.Get("limit"), 20)
	if err != nil || limit < 1 || limit > 100 {
		WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "limit must be an integer between 1 and 100", nil)
		return
	}
	offset := (page - 1) * limit

	products, total, err := s.store.ListProducts(r.Context(), search, limit, offset)
	if err != nil {
		log.Printf("list products: %v", err)
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", nil)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"data":  products,
		"page":  page,
		"limit": limit,
		"total": total,
	})
}

func (s *Server) adjustStock(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "id must be an integer", nil)
		return
	}

	var req adjustStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", "request body must be a valid JSON", nil)
		return
	}

	if req.Type != "in" && req.Type != "out" {
		WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "type must be 'in' or 'out'", nil)
		return
	}
	if req.Qty <= 0 {
		WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "qty must be greater than 0", nil)
		return
	}

	newStock, err := s.store.AdjustStock(r.Context(), id, req.Type, req.Qty, req.Note)
	if err != nil {
		var insuff *store.InsufficientStockError

		if errors.Is(err, store.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "product not found", nil)
			return
		}
		if errors.As(err, &insuff) {
			WriteError(w, http.StatusConflict, "INSUFFICIENT_STOCK", "insufficient stock",
				map[string]any{"available": insuff.Available})
			return
		}

		log.Printf("adjust stock: %v", err)
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", nil)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"id":    id,
		"stock": newStock,
	})
}

func (s *Server) listMovements(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "id must be an integer", nil)
		return
	}

	movements, err := s.store.ListMovements(r.Context(), id)
	if err != nil {
		log.Printf("list movements: %v", err)
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", nil)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"data": movements})
}

func (s *Server) createOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", "request body must be valid JSON", nil)
		return
	}

	var details []string

	if strings.TrimSpace(req.Customer) == "" {
		details = append(details, "customer is required")
	}
	if len(req.Items) == 0 {
		details = append(details, "items must not be empty")
	}

	seen := make(map[string]bool)
	for i, it := range req.Items {
		if it.SKU == "" {
			details = append(details, fmt.Sprintf("items[%d].sku is required", i))
		} else if seen[it.SKU] {
			details = append(details, fmt.Sprintf("items[%d].sku %q is duplicated", i, it.SKU))
		}
		seen[it.SKU] = true

		if it.Qty <= 0 {
			details = append(details, fmt.Sprintf("items[%d].qty must be greater than 0", i))
		}
	}

	if len(details) > 0 {
		WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid order", map[string]any{"details": details})
		return
	}

	order, err := s.store.CreateOrder(r.Context(), req.Customer, req.Items)
	if err != nil {
		var unknown *store.UnknownSKUError
		var inactive *store.InactiveProductError
		var insuff *store.InsufficientStockError

		switch {
		case errors.As(err, &unknown):
			WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid order",
				map[string]any{"details": []string{"unknown sku: " + strings.Join(unknown.SKUs, ", ")}})
		case errors.As(err, &inactive):
			WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid order",
				map[string]any{"details": []string{"inactive product: " + strings.Join(inactive.SKUs, ", ")}})
		case errors.As(err, &insuff):
			WriteError(w, http.StatusConflict, "INSUFFICIENT_STOCK", "insufficient stock",
				map[string]any{"sku": insuff.SKU, "available": insuff.Available})
		default:
			log.Printf("create order: %v", err)
			WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", nil)
		}
		return
	}

	WriteJSON(w, http.StatusCreated, order)

}

func intParam(raw string, def int) (int, error) {
	if raw == "" {
		return def, nil
	}
	return strconv.Atoi(raw)
}
