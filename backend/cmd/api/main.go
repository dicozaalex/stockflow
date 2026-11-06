package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/dicozaalex/stockflow/backend/internal/httpapi"
	"github.com/dicozaalex/stockflow/backend/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()
	// open db pool connection
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://stockflowuser:password123@localhost:5433/stockflowdb"
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Cannot create db pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("cannot reach database: %v", err)
	}
	log.Println("connected to database")

	st := store.New(pool)
	srv := httpapi.NewServer(st)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("listening on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, srv.Routes()))
}
