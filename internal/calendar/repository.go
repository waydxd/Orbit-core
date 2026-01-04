package calendar

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/waydxd/Orbit-core/internal/shared/database"
	"github.com/waydxd/Orbit-core/internal/shared/models"
)

// EventRepository defines database operations for events
type EventRepository interface {
	CreateEvent(ctx context.Context, event *models.Event) error
	GetEventByID(ctx context.Context, id string) (*models.Event, error)
	ListEvents(ctx context.Context, userID string, startTime, endTime time.Time) ([]*models.Event, error)
	UpdateEvent(ctx context.Context, event *models.Event) error
	DeleteEvent(ctx context.Context, id string) error
}

// TaskRepository defines database operations for tasks
type TaskRepository interface {
	CreateTask(ctx context.Context, task *models.Task) error
	GetTaskByID(ctx context.Context, id string) (*models.Task, error)
	ListTasks(ctx context.Context, userID string, completed *bool) ([]*models.Task, error)
	UpdateTask(ctx context.Context, task *models.Task) error
	DeleteTask(ctx context.Context, id string) error
}

// SQLEventRepository implements EventRepository using PostgreSQL
type SQLEventRepository struct {
	db *database.DB
}

// SQLTaskRepository implements TaskRepository using PostgreSQL
type SQLTaskRepository struct {
	db *database.DB
}

// NewSQLEventRepository creates a new SQL event repository
func NewSQLEventRepository(db *database.DB) EventRepository {
	return &SQLEventRepository{db: db}
}

// NewSQLTaskRepository creates a new SQL task repository
func NewSQLTaskRepository(db *database.DB) TaskRepository {
	return &SQLTaskRepository{db: db}
}

// ===== Event Repository Implementation =====

// CreateEvent inserts a new event into the database
func (r *SQLEventRepository) CreateEvent(ctx context.Context, event *models.Event) error {
	query := `
		INSERT INTO events (id, user_id, title, description, start_time, end_time, location, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.ExecContext(ctx, query,
		event.ID,
		event.UserID,
		event.Title,
		event.Description,
		event.StartTime,
		event.EndTime,
		event.Location,
		event.CreatedAt,
		event.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}
	return nil
}

// GetEventByID retrieves an event by ID
func (r *SQLEventRepository) GetEventByID(ctx context.Context, id string) (*models.Event, error) {
	query := `
		SELECT id, user_id, title, description, start_time, end_time, location, created_at, updated_at
		FROM events WHERE id = $1
	`
	event := &models.Event{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&event.ID,
		&event.UserID,
		&event.Title,
		&event.Description,
		&event.StartTime,
		&event.EndTime,
		&event.Location,
		&event.CreatedAt,
		&event.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("event not found")
		}
		return nil, fmt.Errorf("failed to get event: %w", err)
	}
	return event, nil
}

// ListEvents retrieves events for a user within a time range
func (r *SQLEventRepository) ListEvents(ctx context.Context, userID string, startTime, endTime time.Time) ([]*models.Event, error) {
	var rows *sql.Rows
	var err error

	if userID == "" {
		query := `
			SELECT id, user_id, title, description, start_time, end_time, location, created_at, updated_at
			FROM events
			WHERE start_time >= $1 AND end_time <= $2
			ORDER BY start_time
		`
		rows, err = r.db.QueryContext(ctx, query, startTime, endTime)
	} else {
		query := `
			SELECT id, user_id, title, description, start_time, end_time, location, created_at, updated_at
			FROM events
			WHERE user_id = $1 AND start_time >= $2 AND end_time <= $3
			ORDER BY start_time
		`
		rows, err = r.db.QueryContext(ctx, query, userID, startTime, endTime)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Printf("failed to close rows: %v\n", err)
		}
	}(rows)

	var events []*models.Event
	for rows.Next() {
		event := &models.Event{}
		err := rows.Scan(
			&event.ID,
			&event.UserID,
			&event.Title,
			&event.Description,
			&event.StartTime,
			&event.EndTime,
			&event.Location,
			&event.CreatedAt,
			&event.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, event)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating events: %w", err)
	}

	return events, nil
}

// UpdateEvent updates an existing event
func (r *SQLEventRepository) UpdateEvent(ctx context.Context, event *models.Event) error {
	query := `
		UPDATE events
		SET title = $1, description = $2, start_time = $3, end_time = $4, location = $5, updated_at = $6
		WHERE id = $7
	`
	_, err := r.db.ExecContext(ctx, query,
		event.Title,
		event.Description,
		event.StartTime,
		event.EndTime,
		event.Location,
		time.Now(),
		event.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update event: %w", err)
	}
	return nil
}

// DeleteEvent deletes an event
func (r *SQLEventRepository) DeleteEvent(ctx context.Context, id string) error {
	query := "DELETE FROM events WHERE id = $1"
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}
	return nil
}

// ===== Task Repository Implementation =====

// CreateTask inserts a new task into the database
func (r *SQLTaskRepository) CreateTask(ctx context.Context, task *models.Task) error {
	query := `
		INSERT INTO tasks (id, user_id, title, description, due_date, completed, priority, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.ExecContext(ctx, query,
		task.ID,
		task.UserID,
		task.Title,
		task.Description,
		task.DueDate,
		task.Completed,
		task.Priority,
		task.CreatedAt,
		task.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}
	return nil
}

// GetTaskByID retrieves a task by ID
func (r *SQLTaskRepository) GetTaskByID(ctx context.Context, id string) (*models.Task, error) {
	query := `
		SELECT id, user_id, title, description, due_date, completed, priority, created_at, updated_at
		FROM tasks WHERE id = $1
	`
	task := &models.Task{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&task.ID,
		&task.UserID,
		&task.Title,
		&task.Description,
		&task.DueDate,
		&task.Completed,
		&task.Priority,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	return task, nil
}

// ListTasks retrieves tasks for a user
func (r *SQLTaskRepository) ListTasks(ctx context.Context, userID string, completed *bool) ([]*models.Task, error) {
	query := `
		SELECT id, user_id, title, description, due_date, completed, priority, created_at, updated_at
		FROM tasks
		WHERE user_id = $1
	`
	args := []interface{}{userID}

	if completed != nil {
		query += " AND completed = $2"
		args = append(args, *completed)
	}

	query += " ORDER BY due_date ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Printf("failed to close rows: %v\n", err)
		}
	}(rows)

	var tasks []*models.Task
	for rows.Next() {
		task := &models.Task{}
		err := rows.Scan(
			&task.ID,
			&task.UserID,
			&task.Title,
			&task.Description,
			&task.DueDate,
			&task.Completed,
			&task.Priority,
			&task.CreatedAt,
			&task.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		tasks = append(tasks, task)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tasks: %w", err)
	}

	return tasks, nil
}

// UpdateTask updates an existing task
func (r *SQLTaskRepository) UpdateTask(ctx context.Context, task *models.Task) error {
	query := `
		UPDATE tasks
		SET title = $1, description = $2, due_date = $3, completed = $4, priority = $5, updated_at = $6
		WHERE id = $7
	`
	_, err := r.db.ExecContext(ctx, query,
		task.Title,
		task.Description,
		task.DueDate,
		task.Completed,
		task.Priority,
		time.Now(),
		task.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}
	return nil
}

// DeleteTask deletes a task
func (r *SQLTaskRepository) DeleteTask(ctx context.Context, id string) error {
	query := "DELETE FROM tasks WHERE id = $1"
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}
	return nil
}
