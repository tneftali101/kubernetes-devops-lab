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
	http.HandleFunc("POST /api/v1/projects/{id}/tasks", createTaskHandler)
	http.HandleFunc("GET /api/v1/projects/{id}/tasks", getTasksHandler)
	http.HandleFunc("PATCH /api/v1/projects/{id}/tasks/{taskID}", updateTaskHandler)

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

type CreateTaskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	DueDate     string `json:"due_date"`
}

func createTaskHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	projectID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

	if err != nil || projectID <= 0 {
		http.Error(w, `{"error":"invalid project ID"}`, http.StatusBadRequest)
		return
	}

	var task CreateTaskRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&task); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	task.Title = strings.TrimSpace(task.Title)

	if task.Title == "" || len(task.Title) > 255 {
		http.Error(w, `{"error":"title must be 1-255 characters"}`, http.StatusBadRequest)
		return
	}

	if task.Status == "" {
		task.Status = "todo"
	}

	if len(task.Status) > 50 {
		http.Error(w, `{"error":"status is too long"}`, http.StatusBadRequest)
		return
	}

	var dueDate interface{}

	if task.DueDate != "" {
		date, err := time.Parse("2006-01-02", task.DueDate)

		if err != nil {
			http.Error(w, `{"error":"due_date must use YYYY-MM-DD"}`, http.StatusBadRequest)
			return
		}

		dueDate = date.Format("2006-01-02")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var exists bool

	err = db.QueryRowContext(
		ctx,
		"SELECT EXISTS(SELECT 1 FROM projects WHERE id = $1)",
		projectID,
	).Scan(&exists)

	if err != nil {
		log.Printf("Failed to check project: %v", err)
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	if !exists {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}

	var taskID int64

	err = db.QueryRowContext(ctx, `
		INSERT INTO tasks
			(project_id, title, description, status, due_date)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`,
		projectID,
		task.Title,
		task.Description,
		task.Status,
		dueDate,
	).Scan(&taskID)

	if err != nil {
		log.Printf("Failed to create task: %v", err)
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":          taskID,
		"project_id":  projectID,
		"title":       task.Title,
		"description": task.Description,
		"status":      task.Status,
		"due_date":    task.DueDate,
	})
}

type Task struct {
	ID          int64  `json:"id"`
	ProjectID   int64  `json:"project_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	DueDate     string `json:"due_date"`
}

func getTasksHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	projectID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

	if err != nil || projectID <= 0 {
		http.Error(w, `{"error":"invalid project ID"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Verify that the project exists.
	var exists bool

	err = db.QueryRowContext(
		ctx,
		"SELECT EXISTS(SELECT 1 FROM projects WHERE id = $1)",
		projectID,
	).Scan(&exists)

	if err != nil {
		log.Printf("Failed to check project: %v", err)
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	if !exists {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}

	// Retrieve all tasks belonging to the project.
	rows, err := db.QueryContext(ctx, `
		SELECT id, project_id, title,
		       COALESCE(description, ''),
		       status,
		       COALESCE(TO_CHAR(due_date, 'YYYY-MM-DD'), '')
		FROM tasks
		WHERE project_id = $1
		ORDER BY id ASC
	`, projectID)

	if err != nil {
		log.Printf("Failed to retrieve tasks: %v", err)
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tasks := []Task{}

	for rows.Next() {
		var task Task

		err := rows.Scan(
			&task.ID,
			&task.ProjectID,
			&task.Title,
			&task.Description,
			&task.Status,
			&task.DueDate,
		)

		if err != nil {
			log.Printf("Failed to scan task: %v", err)
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Failed while reading tasks: %v", err)
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(tasks)
}

type UpdateTaskRequest struct {
	Status string `json:"status"`
}

func updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	projectID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || projectID <= 0 {
		http.Error(w, `{"error":"invalid project ID"}`, http.StatusBadRequest)
		return
	}

	taskID, err := strconv.ParseInt(r.PathValue("taskID"), 10, 64)
	if err != nil || taskID <= 0 {
		http.Error(w, `{"error":"invalid task ID"}`, http.StatusBadRequest)
		return
	}

	var request UpdateTaskRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	// Only allow supported task statuses.
	switch request.Status {
	case "todo", "in_progress", "completed":
		// Valid status.
	default:
		http.Error(w, `{"error":"invalid task status"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var task Task

	err = db.QueryRowContext(ctx, `
        UPDATE tasks
        SET status = $1,
            updated_at = NOW()
        WHERE id = $2 AND project_id = $3
        RETURNING id, project_id, title,
                  COALESCE(description, ''),
                  status,
                  COALESCE(TO_CHAR(due_date, 'YYYY-MM-DD'), '')
    `,
		request.Status,
		taskID,
		projectID,
	).Scan(
		&task.ID,
		&task.ProjectID,
		&task.Title,
		&task.Description,
		&task.Status,
		&task.DueDate,
	)

	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"task not found"}`, http.StatusNotFound)
		return
	}

	if err != nil {
		log.Printf("Failed to update task: %v", err)
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(task)
}
