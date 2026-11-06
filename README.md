# StockFlow

StockFlow is a small inventory and sales-order system. It tracks products and their stock levels, records every stock change as a movement, and creates sales orders that decrement stock atomically.

**Stack:** Go 1.25.0 (`net/http` + `pgx`) for the API, PostgreSQL 16 in Docker, and React + TypeScript (Vite) for the frontend.


## How to run

### Prerequisites

- Docker (with Docker Compose)
- Go 1.25+
- Node.js 20.19+

### 1. Start the database

```bash
docker compose up -d
```

- **PostgreSQL** is exposed on host port **5433** (not 5432, to avoid clashing with a locally installed Postgres).
- The schema and seed data are applied automatically the first time the volume is created. 

- **Adminer** (DB web UI) runs at http://localhost:5050:
  - System: `PostgreSQL`
  - Server: `db`
  - Username: `stockflowuser` 
  - Password: `password123` 
  - Database: `stockflowdb`

### 2. Start the backend

```bash
cd backend
go run ./cmd/api
```

The API listens on http://localhost:8080. Optional environment variables:

| Variable       | Default                                                               |
|----------------|-----------------------------------------------------------------------|
| `DATABASE_URL` | `postgres://stockflowuser:password123@localhost:5433/stockflowdb` |
| `PORT`         | `8080`                                                                |

The server pings the database on startup and exits with a clear error if it cannot connect.

### 3. Start the frontend

```bash
cd frontend
npm install
npm run dev
```

Open http://localhost:5173.

**Dev proxy:** the Vite dev server forwards every request under `/api` to `http://localhost:8080` (see `frontend/vite.config.ts`). From the browser's point of view the frontend and the API share one origin, so the backend needs no CORS middleware. The backend must be running for the frontend to load data.

### 4. Run the tests

The database must be running.

```bash
cd backend
go test ./... -v -count=1
```

`-count=1` disables Go's test cache, since the tests depend on database state rather than only on code.

### 5. Reset all data

```bash
docker compose down -v
docker compose up -d
```

`-v` removes the database volume, so the schema and seed are applied again from scratch.


## API

| Method | Path                           | Description                                          |
|--------|--------------------------------|------------------------------------------------------|
| GET    | `/api/health`                  | 200 if API and DB are up, 503 otherwise              |
| GET    | `/api/products`                | List products (`search`, `page`, `limit`)            |
| POST   | `/api/products/{id}/stock`     | Adjust stock (`in` / `out`)                          |
| GET    | `/api/products/{id}/movements` | Movement history, newest first                       |
| POST   | `/api/orders`                  | Create a sales order                                 |

Errors use one envelope: `{"error": {"code": "...", "message": "...", ...extra}}`.

Examples (bash, fresh seed data):

```bash
# List products → 200 {data, page, limit, total}
curl -s "localhost:8080/api/products?search=mouse&page=1&limit=20"

# Adjust stock → 200 {"id":8,"stock":5}
curl -s -X POST localhost:8080/api/products/8/stock \
  -H "Content-Type: application/json" -d '{"type":"out","qty":3,"note":"damaged"}'

# Create order → 201 with items, unit prices and server-computed total
curl -s -X POST localhost:8080/api/orders \
  -H "Content-Type: application/json" \
  -d '{"customer":"Andi","items":[{"sku":"ELC-004","qty":2},{"sku":"ELC-006","qty":1}]}'

# Insufficient stock → 409 {"error":{"code":"INSUFFICIENT_STOCK",...,"sku":"ELC-009","available":5}}, nothing written
curl -s -X POST localhost:8080/api/orders \
  -H "Content-Type: application/json" \
  -d '{"customer":"Andi","items":[{"sku":"ELC-004","qty":1},{"sku":"ELC-009","qty":6}]}'
```

## Frontend

The Products page (http://localhost:5173) supports:

- **Search** by SKU or name, debounced by 300 ms so a request is sent only after typing stops.
- **Filter:** All / Active / Inactive / Low stock (≤ 10).
- **Loading, empty and error states.** If the API is unreachable, an error message is shown with a **Retry** button.
- **Table** with SKU, name, price (formatted as `Rp 1.234.567`), stock, active/inactive badge, and last movement as relative time ("2h ago").
- **Adjust stock:** clicking **Adjust** opens a form below the table (direction, qty, optional note). Qty must be an integer greater than 0, and the Save button is disabled while the request is in flight. If the backend responds with 409, the form shows `Insufficient stock, available: N`. On success the form closes and the list reloads.

All `fetch` calls and error-envelope parsing live in `frontend/src/api.ts`; components never build URLs themselves.


## Design decisions

- **Stock is guarded inside the UPDATE, within a transaction.** Adjustments and orders use a conditional UPDATE (`WHERE stock >= qty`) and write the movement in the same transaction. Postgres's row lock makes the check and the decrement a single step, so concurrent requests can never push stock below zero. There is no SELECT-then-UPDATE.
- **Order items are locked in SKU order** to prevent deadlocks between orders that share products.
- **Orders are fully validated before anything is written.** All validation errors are returned together in `details`, and unknown or inactive SKUs are rejected with 400 before any stock changes.
- **Prices come from the database.** The client sends only `sku` and `qty`; `unit_price` and `total` are computed server-side.
- **`last_movement_at` is derived** from `MAX(created_at)` in `stock_movements` rather than stored, so it cannot drift from the real history.
- **Every stock change is a movement** (seed, adjustments, orders), keeping `stock = SUM(in) − SUM(out)`. Cancel (planned) would write an `in` movement instead of deleting history.
- **Error bodies use a map** for optional extra fields (`available`, `sku`, `details`). Flexible, but not type-checked.
- **Postgres runs on host port 5433** to avoid clashing with a local install.

### Testing

- Store tests run against the real Postgres (transactions and rollback are what's being tested) and cover (a) insufficient stock rejected with nothing written and (b) the happy path writing a movement and updating stock.
- Concurrency was verified manually: 10 parallel orders for a product with stock 5 gave 5 × 201, 5 × 409, and a final stock of 0.
- `go test -race` was not run because it requires cgo, which isn't available in this Windows environment.


## Not implemented / what I'd do next

### Missing features

- **Product CRUD:** `POST /api/products`, `PUT /api/products/{id}`, `DELETE /api/products/{id}` (including the 409 `PRODUCT_HAS_HISTORY` rule and 204 on success).
- **Order cancel:** `POST /api/orders/{id}/cancel`. Planned approach, in one transaction:
  1. `UPDATE sales_orders SET status = 'CANCELLED', cancelled_at = now() WHERE id = $1 AND status = 'CREATED' RETURNING id`. The status condition makes a double cancel impossible, even concurrently. If no row matches, a follow-up SELECT distinguishes 404 (no such order) from 409 (already cancelled).
  2. Load the order's items, sort them by SKU (same lock order as order creation).
  3. For each item, restore stock with `stock = stock + qty` and insert an `in` movement with the note `cancel order #id`.
  4. Commit.
- **Order detail:** `GET /api/orders/{id}` with items, status and total.
- **Frontend:** create-order flow (highlighting the failed SKU on 409 and listing `details` on 400), order detail page with cancel confirmation, and summary cards (total products, active products, total stock value, low-stock count).
- **Frontend tests (Vitest):** the qty validation and the `timeAgo` / price formatting helpers are pure functions and the obvious first candidates.

### Improvements

1. Add an index on `stock_movements (product_id, created_at)` to speed up the movements list and the `last_movement_at` subquery.
2. Escape LIKE wildcards (`%`, `_`) in `search`. Currently `search=%` matches every product.
3. Add small handler helpers (for example, one for validation errors that only takes a message) to cut down on repeated `WriteError(w, 400, "VALIDATION_ERROR", ..., nil)` calls. The `return` still has to stay in the handler.
4. Replace the error-body map with a typed struct. Note that `available` would need to be a pointer (`*int`), because `omitempty` would otherwise hide a legitimate `available: 0`.
5. Return 404 from `GET /api/products/{id}/movements` when the product does not exist, instead of an empty list.
6. Widen `price` and `total` to `BIGINT`. With `INTEGER`, a very large order (for example 300 × Rp 8,500,000) overflows and fails with a 500.


## Hours spent

| Session          | Time                  |
|------------------|-----------------------|
| Thursday evening | ~ 2.5 hours           | 
| Friday           | ~ 10 hours            | 
| **Total**        | ~ 12.5 hours          | 