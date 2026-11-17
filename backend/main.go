package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var db *sql.DB

func getEnv(key, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}

func main() {
	port := getEnv("SERVER_PORT", "8080")

	dbHost := getEnv("DATABASE_HOST", "127.0.0.1")
	dbPort := getEnv("DATABASE_PORT", "5432")
	dbUser := getEnv("DATABASE_USER", "creator_studio")
	dbName := getEnv("DATABASE_NAME", "creator_studio")

	connectionString := fmt.Sprintf(
		"host=%s port=%s user=%s dbname=%s sslmode=disable",
		dbHost,
		dbPort,
		dbUser,
		dbName,
	)

	var err error

	db, err = sql.Open("pgx", connectionString)
	if err != nil {
		log.Fatal("Database configuration failed: ", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/ready", readyHandler)
	http.HandleFunc("/api/v1/status", statusHandler)

	fmt.Printf("Creator Studio API running on port %s\n", port)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)

		json.NewEncoder(w).Encode(map[string]string{
			"status": "not_ready",
			"reason": "database unavailable",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status":   "ready",
		"database": "connected",
	})
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]string{
		"application": "Creator Studio",
		"environment": getEnv("APP_ENV", "development"),
		"version":     "0.1.0",
		"status":      "running",
	})
}
