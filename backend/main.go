package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	http.HandleFunc("POST /api/v1/projects", createProjectHandler)
	http.HandleFunc("GET /api/v1/projects", getProjectsHandler)
	http.HandleFunc("GET /api/v1/projects/{id}", getProjectByIDHandler)

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

type CreateProjectRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Platform    string `json:"platform"`
	Status      string `json:"status"`
}

func createProjectHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var project CreateProjectRequest

	if err := json.NewDecoder(r.Body).Decode(&project); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	project.Title = strings.TrimSpace(project.Title)

	if project.Title == "" || len(project.Title) > 255 {
		http.Error(w, `{"error":"title must be 1-255 characters"}`, http.StatusBadRequest)
		return
	}

	if len(project.Platform) > 50 ||
		len(project.Status) > 50 {
		http.Error(w, `{"error":"platform or status is too long"}`, http.StatusBadRequest)
		return
	}

	if project.Status == "" {
		project.Status = "planned"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var id int64

	err := db.QueryRowContext(
		ctx,
		`INSERT INTO projects
			(title, description, platform, status)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		project.Title,
		project.Description,
		project.Platform,
		project.Status,
	).Scan(&id)

	if err != nil {
		log.Printf("Failed to create project: %v", err)
		http.Error(w, `{"error":"database error"}`,
			http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":          id,
		"title":       project.Title,
		"description": project.Description,
		"platform":    project.Platform,
		"status":      project.Status,
	})
}

type Project struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Platform    string `json:"platform"`
	Status      string `json:"status"`
}

func getProjectsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, `
		SELECT id, title,
		       COALESCE(description, ''),
		       COALESCE(platform, ''),
		       status
		FROM projects
		ORDER BY id ASC
	`)

	if err != nil {
		log.Printf("Failed to retrieve projects: %v", err)
		http.Error(w, `{"error":"database error"}`,
			http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	projects := []Project{}

	for rows.Next() {
		var project Project

		err := rows.Scan(
			&project.ID,
			&project.Title,
			&project.Description,
			&project.Platform,
			&project.Status,
		)

		if err != nil {
			log.Printf("Failed to scan project: %v", err)
			http.Error(w, `{"error":"database error"}`,
				http.StatusInternalServerError)
			return
		}

		projects = append(projects, project)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Failed while reading projects: %v", err)
		http.Error(w, `{"error":"database error"}`,
			http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(projects)
}

func getProjectByIDHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

	if err != nil || id <= 0 {
		http.Error(w, `{"error":"invalid project ID"}`,
			http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var project Project

	err = db.QueryRowContext(ctx, `
		SELECT id, title,
		       COALESCE(description, ''),
		       COALESCE(platform, ''),
		       status
		FROM projects
		WHERE id = $1
	`, id).Scan(
		&project.ID,
		&project.Title,
		&project.Description,
		&project.Platform,
		&project.Status,
	)

	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"project not found"}`,
			http.StatusNotFound)
		return
	}

	if err != nil {
		log.Printf("Failed to retrieve project: %v", err)
		http.Error(w, `{"error":"database error"}`,
			http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(project)
}
