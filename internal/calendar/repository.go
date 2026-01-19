package calendar

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/waydxd/Orbit-core/internal/shared/database"
	"github.com/waydxd/Orbit-core/internal/shared/database/db"
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
	queries *db.Queries
	pool    *database.DB
}

// SQLTaskRepository implements TaskRepository using PostgreSQL
type SQLTaskRepository struct {
	queries *db.Queries
	pool    *database.DB
}

// NewSQLEventRepository creates a new SQL event repository
func NewSQLEventRepository(pool *database.DB) EventRepository {
	return &SQLEventRepository{
		queries: db.New(pool.Pool),
		pool:    pool,
	}
}

// NewSQLTaskRepository creates a new SQL task repository
func NewSQLTaskRepository(pool *database.DB) TaskRepository {
	return &SQLTaskRepository{
		queries: db.New(pool.Pool),
		pool:    pool,
	}
}

// ===== Event Repository Implementation =====

// CreateEvent inserts a new event into the database
func (r *SQLEventRepository) CreateEvent(ctx context.Context, event *models.Event) error {
	params := db.CreateEventParams{
		ID:          database.StringToUUID(event.ID),
		UserID:      database.StringToUUID(event.UserID),
		Title:       event.Title,
		Description: database.StringToText(event.Description),
		StartTime:   database.TimeToTimestamptz(event.StartTime),
		EndTime:     database.TimeToTimestamptz(event.EndTime),
		Location:    database.StringToText(event.Location),
		CreatedAt:   database.TimeToTimestamptz(event.CreatedAt),
		UpdatedAt:   database.TimeToTimestamptz(event.UpdatedAt),
	}

	err := r.queries.CreateEvent(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}
	return nil
}

// GetEventByID retrieves an event by ID
func (r *SQLEventRepository) GetEventByID(ctx context.Context, id string) (*models.Event, error) {
	row, err := r.queries.GetEventByID(ctx, database.StringToUUID(id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("event not found")
		}
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	return &models.Event{
		ID:          database.UUIDToString(row.ID),
		UserID:      database.UUIDToString(row.UserID),
		Title:       row.Title,
		Description: database.TextToString(row.Description),
		StartTime:   database.TimestamptzToTime(row.StartTime),
		EndTime:     database.TimestamptzToTime(row.EndTime),
		Location:    database.TextToString(row.Location),
		CreatedAt:   database.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:   database.TimestamptzToTime(row.UpdatedAt),
	}, nil
}

// ListEvents retrieves events for a user within a time range
func (r *SQLEventRepository) ListEvents(ctx context.Context, userID string, startTime, endTime time.Time) ([]*models.Event, error) {
	var rows []db.Event
	var err error

	sTime := database.TimeToTimestamptz(startTime)
	eTime := database.TimeToTimestamptz(endTime)

	if userID == "" {
		params := db.ListEventsByTimeParams{
			StartTime: sTime,
			EndTime:   eTime,
		}
		// NOTE: I generated ListEventsByTime, checking if it matches expectation
		rows, err = r.queries.ListEventsByTime(ctx, params)
	} else {
		params := db.ListEventsByUserAndTimeParams{
			UserID:    database.StringToUUID(userID),
			StartTime: sTime,
			EndTime:   eTime,
		}
		rows, err = r.queries.ListEventsByUserAndTime(ctx, params)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	var events []*models.Event
	for _, row := range rows {
		events = append(events, &models.Event{
			ID:          database.UUIDToString(row.ID),
			UserID:      database.UUIDToString(row.UserID),
			Title:       row.Title,
			Description: database.TextToString(row.Description),
			StartTime:   database.TimestamptzToTime(row.StartTime),
			EndTime:     database.TimestamptzToTime(row.EndTime),
			Location:    database.TextToString(row.Location),
			CreatedAt:   database.TimestamptzToTime(row.CreatedAt),
			UpdatedAt:   database.TimestamptzToTime(row.UpdatedAt),
		})
	}

	return events, nil
}

// UpdateEvent updates an existing event
func (r *SQLEventRepository) UpdateEvent(ctx context.Context, event *models.Event) error {
	params := db.UpdateEventParams{
		Title:       event.Title,
		Description: database.StringToText(event.Description),
		StartTime:   database.TimeToTimestamptz(event.StartTime),
		EndTime:     database.TimeToTimestamptz(event.EndTime),
		Location:    database.StringToText(event.Location),
		UpdatedAt:   database.TimeToTimestamptz(time.Now()),
		ID:          database.StringToUUID(event.ID),
	}

	err := r.queries.UpdateEvent(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to update event: %w", err)
	}
	return nil
}

// DeleteEvent deletes an event
func (r *SQLEventRepository) DeleteEvent(ctx context.Context, id string) error {
	err := r.queries.DeleteEvent(ctx, database.StringToUUID(id))
	if err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}
	return nil
}

// ===== Task Repository Implementation =====

// CreateTask inserts a new task into the database
func (r *SQLTaskRepository) CreateTask(ctx context.Context, task *models.Task) error {
	params := db.CreateTaskParams{
		ID:          database.StringToUUID(task.ID),
		UserID:      database.StringToUUID(task.UserID),
		Title:       task.Title,
		Description: database.StringToText(task.Description),
		DueDate:     database.TimeToTimestamptz(task.DueDate),
		Completed:   pgtype.Bool{Bool: task.Completed, Valid: true},
		Priority:    pgtype.Text{String: task.Priority, Valid: task.Priority != ""},
		CreatedAt:   database.TimeToTimestamptz(task.CreatedAt),
		UpdatedAt:   database.TimeToTimestamptz(task.UpdatedAt),
	}

	err := r.queries.CreateTask(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}
	return nil
}

// GetTaskByID retrieves a task by ID
func (r *SQLTaskRepository) GetTaskByID(ctx context.Context, id string) (*models.Task, error) {
	row, err := r.queries.GetTaskByID(ctx, database.StringToUUID(id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	return &models.Task{
		ID:          database.UUIDToString(row.ID),
		UserID:      database.UUIDToString(row.UserID),
		Title:       row.Title,
		Description: database.TextToString(row.Description),
		DueDate:     database.TimestamptzToTime(row.DueDate),
		Completed:   row.Completed.Bool,
		Priority:    database.TextToString(row.Priority),
		CreatedAt:   database.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:   database.TimestamptzToTime(row.UpdatedAt),
	}, nil
}

// ListTasks retrieves tasks for a user
func (r *SQLTaskRepository) ListTasks(ctx context.Context, userID string, completed *bool) ([]*models.Task, error) {
	params := db.ListTasksParams{
		UserID: database.StringToUUID(userID),
	}

	if completed != nil {
		params.Completed = pgtype.Bool{Bool: *completed, Valid: true}
	} else {
		params.Completed = pgtype.Bool{Valid: false}
	}

	rows, err := r.queries.ListTasks(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	var tasks []*models.Task
	for _, row := range rows {
		tasks = append(tasks, &models.Task{
			ID:          database.UUIDToString(row.ID),
			UserID:      database.UUIDToString(row.UserID),
			Title:       row.Title,
			Description: database.TextToString(row.Description),
			DueDate:     database.TimestamptzToTime(row.DueDate),
			Completed:   row.Completed.Bool,
			Priority:    database.TextToString(row.Priority),
			CreatedAt:   database.TimestamptzToTime(row.CreatedAt),
			UpdatedAt:   database.TimestamptzToTime(row.UpdatedAt),
		})
	}

	return tasks, nil
}

// UpdateTask updates an existing task
func (r *SQLTaskRepository) UpdateTask(ctx context.Context, task *models.Task) error {
	params := db.UpdateTaskParams{
		Title:       task.Title,
		Description: database.StringToText(task.Description),
		DueDate:     database.TimeToTimestamptz(task.DueDate),
		Completed:   pgtype.Bool{Bool: task.Completed, Valid: true},
		Priority:    pgtype.Text{String: task.Priority, Valid: task.Priority != ""},
		UpdatedAt:   database.TimeToTimestamptz(time.Now()),
		ID:          database.StringToUUID(task.ID),
	}

	err := r.queries.UpdateTask(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}
	return nil
}

// DeleteTask deletes a task
func (r *SQLTaskRepository) DeleteTask(ctx context.Context, id string) error {
	err := r.queries.DeleteTask(ctx, database.StringToUUID(id))
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}
	return nil
}
