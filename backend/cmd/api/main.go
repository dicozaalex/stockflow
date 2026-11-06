package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/dicozaalex/stockflow/backend/internal/httpapi"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// open db pool connection
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://stockflowuser:password123@localhost:5433/stockflowdb"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Cannot create db pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("cannot reach database: %v", err)
	}
	log.Println("connected to database")

	mux := http.NewServeMux()
	// api healthcheck
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			httpapi.WriteError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "Database is unreachable", nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("listening on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
