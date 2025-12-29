package habit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/waydxd/Orbit-core/internal/shared/database"
	"github.com/waydxd/Orbit-core/internal/shared/models"
)

// Repository defines database operations for habit tracking
type Repository interface {
	// Event frequency operations
	UpsertEventFrequency(ctx context.Context, freq *models.EventFrequency) error
	GetEventFrequencyByPattern(ctx context.Context, userID, title string, durationMinutes, timeOfDay, dayOfWeek int) (*models.EventFrequency, error)
	GetEventFrequenciesAboveThreshold(ctx context.Context, userID string, threshold int) ([]*models.EventFrequency, error)
	UpdateEventFrequency(ctx context.Context, freq *models.EventFrequency) error

	// Habit suggestion operations
	CreateHabitSuggestion(ctx context.Context, suggestion *models.HabitSuggestion) error
	GetPendingHabitSuggestions(ctx context.Context, userID string) ([]*models.HabitSuggestion, error)
	GetHabitSuggestionByID(ctx context.Context, id string) (*models.HabitSuggestion, error)
	UpdateHabitSuggestionStatus(ctx context.Context, id, status string, recurrenceEndDate *time.Time) error

	// Recurring event operations
	CreateRecurringEvent(ctx context.Context, recurring *models.RecurringEvent) error
	GetActiveRecurringEvents(ctx context.Context, userID string) ([]*models.RecurringEvent, error)
	GetRecurringEventByID(ctx context.Context, id string) (*models.RecurringEvent, error)
	DeactivateRecurringEvent(ctx context.Context, id string) error
}

// SQLRepository implements Repository using PostgreSQL
type SQLRepository struct {
	db *database.DB
}

// NewSQLRepository creates a new SQL habit repository
func NewSQLRepository(db *database.DB) Repository {
	return &SQLRepository{db: db}
}

// ===== Event Frequency Repository Implementation =====

// UpsertEventFrequency inserts or updates an event frequency record
func (r *SQLRepository) UpsertEventFrequency(ctx context.Context, freq *models.EventFrequency) error {
	// Convert occurrence timestamps to JSON
	timestampsJSON, err := json.Marshal(freq.OccurrenceTimestamps)
	if err != nil {
		return fmt.Errorf("failed to marshal occurrence timestamps: %w", err)
	}

	query := `
		INSERT INTO event_frequency (
			id, user_id, title, description, location, duration_minutes,
			time_of_day, day_of_week, occurrence_count, suggestion_threshold,
			suggestion_shown, habit_accepted, occurrence_timestamps, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (user_id, title, duration_minutes, time_of_day, day_of_week)
		DO UPDATE SET
			occurrence_count = event_frequency.occurrence_count + 1,
			occurrence_timestamps = $13,
			updated_at = $15
		RETURNING id, occurrence_count
	`

	err = r.db.QueryRowContext(ctx, query,
		freq.ID,
		freq.UserID,
		freq.Title,
		freq.Description,
		freq.Location,
		freq.DurationMinutes,
		freq.TimeOfDay,
		freq.DayOfWeek,
		freq.OccurrenceCount,
		freq.SuggestionThreshold,
		freq.SuggestionShown,
		freq.HabitAccepted,
		timestampsJSON,
		freq.CreatedAt,
		freq.UpdatedAt,
	).Scan(&freq.ID, &freq.OccurrenceCount)

	if err != nil {
		return fmt.Errorf("failed to upsert event frequency: %w", err)
	}
	return nil
}

// GetEventFrequencyByPattern retrieves an event frequency by its pattern
func (r *SQLRepository) GetEventFrequencyByPattern(ctx context.Context, userID, title string, durationMinutes, timeOfDay, dayOfWeek int) (*models.EventFrequency, error) {
	query := `
		SELECT id, user_id, title, description, location, duration_minutes,
			   time_of_day, day_of_week, occurrence_count, suggestion_threshold,
			   suggestion_shown, habit_accepted, occurrence_timestamps, created_at, updated_at
		FROM event_frequency
		WHERE user_id = $1 AND title = $2 AND duration_minutes = $3
		      AND time_of_day = $4 AND day_of_week = $5
	`

	freq := &models.EventFrequency{}
	var timestampsJSON []byte

	err := r.db.QueryRowContext(ctx, query, userID, title, durationMinutes, timeOfDay, dayOfWeek).Scan(
		&freq.ID,
		&freq.UserID,
		&freq.Title,
		&freq.Description,
		&freq.Location,
		&freq.DurationMinutes,
		&freq.TimeOfDay,
		&freq.DayOfWeek,
		&freq.OccurrenceCount,
		&freq.SuggestionThreshold,
		&freq.SuggestionShown,
		&freq.HabitAccepted,
		&timestampsJSON,
		&freq.CreatedAt,
		&freq.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get event frequency: %w", err)
	}

	if err := json.Unmarshal(timestampsJSON, &freq.OccurrenceTimestamps); err != nil {
		return nil, fmt.Errorf("failed to unmarshal occurrence timestamps: %w", err)
	}

	return freq, nil
}

// GetEventFrequenciesAboveThreshold retrieves event frequencies that meet or exceed the suggestion threshold
func (r *SQLRepository) GetEventFrequenciesAboveThreshold(ctx context.Context, userID string, threshold int) ([]*models.EventFrequency, error) {
	query := `
		SELECT id, user_id, title, description, location, duration_minutes,
			   time_of_day, day_of_week, occurrence_count, suggestion_threshold,
			   suggestion_shown, habit_accepted, occurrence_timestamps, created_at, updated_at
		FROM event_frequency
		WHERE user_id = $1 AND occurrence_count >= $2 AND suggestion_shown = FALSE AND habit_accepted = FALSE
		ORDER BY occurrence_count DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID, threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to get event frequencies: %w", err)
	}
	defer rows.Close()

	var frequencies []*models.EventFrequency
	for rows.Next() {
		freq := &models.EventFrequency{}
		var timestampsJSON []byte

		err := rows.Scan(
			&freq.ID,
			&freq.UserID,
			&freq.Title,
			&freq.Description,
			&freq.Location,
			&freq.DurationMinutes,
			&freq.TimeOfDay,
			&freq.DayOfWeek,
			&freq.OccurrenceCount,
			&freq.SuggestionThreshold,
			&freq.SuggestionShown,
			&freq.HabitAccepted,
			&timestampsJSON,
			&freq.CreatedAt,
			&freq.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event frequency: %w", err)
		}

		if err := json.Unmarshal(timestampsJSON, &freq.OccurrenceTimestamps); err != nil {
			return nil, fmt.Errorf("failed to unmarshal occurrence timestamps: %w", err)
		}

		frequencies = append(frequencies, freq)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating event frequencies: %w", err)
	}

	return frequencies, nil
}

// UpdateEventFrequency updates an existing event frequency record
func (r *SQLRepository) UpdateEventFrequency(ctx context.Context, freq *models.EventFrequency) error {
	timestampsJSON, err := json.Marshal(freq.OccurrenceTimestamps)
	if err != nil {
		return fmt.Errorf("failed to marshal occurrence timestamps: %w", err)
	}

	query := `
		UPDATE event_frequency
		SET occurrence_count = $1, suggestion_shown = $2, habit_accepted = $3,
			occurrence_timestamps = $4, updated_at = $5
		WHERE id = $6
	`

	_, err = r.db.ExecContext(ctx, query,
		freq.OccurrenceCount,
		freq.SuggestionShown,
		freq.HabitAccepted,
		timestampsJSON,
		time.Now(),
		freq.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update event frequency: %w", err)
	}
	return nil
}

// ===== Habit Suggestion Repository Implementation =====

// CreateHabitSuggestion inserts a new habit suggestion
func (r *SQLRepository) CreateHabitSuggestion(ctx context.Context, suggestion *models.HabitSuggestion) error {
	query := `
		INSERT INTO habit_suggestions (
			id, user_id, event_frequency_id, title, description, location,
			duration_minutes, time_of_day, day_of_week, status, created_at, updated_at, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err := r.db.ExecContext(ctx, query,
		suggestion.ID,
		suggestion.UserID,
		suggestion.EventFrequencyID,
		suggestion.Title,
		suggestion.Description,
		suggestion.Location,
		suggestion.DurationMinutes,
		suggestion.TimeOfDay,
		suggestion.DayOfWeek,
		suggestion.Status,
		suggestion.CreatedAt,
		suggestion.UpdatedAt,
		suggestion.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create habit suggestion: %w", err)
	}
	return nil
}

// GetPendingHabitSuggestions retrieves all pending habit suggestions for a user
func (r *SQLRepository) GetPendingHabitSuggestions(ctx context.Context, userID string) ([]*models.HabitSuggestion, error) {
	query := `
		SELECT id, user_id, event_frequency_id, title, description, location,
			   duration_minutes, time_of_day, day_of_week, status,
			   recurrence_end_date, created_at, updated_at, expires_at
		FROM habit_suggestions
		WHERE user_id = $1 AND status = 'pending' AND expires_at > NOW()
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending habit suggestions: %w", err)
	}
	defer rows.Close()

	var suggestions []*models.HabitSuggestion
	for rows.Next() {
		suggestion := &models.HabitSuggestion{}
		var recurrenceEndDate sql.NullTime

		err := rows.Scan(
			&suggestion.ID,
			&suggestion.UserID,
			&suggestion.EventFrequencyID,
			&suggestion.Title,
			&suggestion.Description,
			&suggestion.Location,
			&suggestion.DurationMinutes,
			&suggestion.TimeOfDay,
			&suggestion.DayOfWeek,
			&suggestion.Status,
			&recurrenceEndDate,
			&suggestion.CreatedAt,
			&suggestion.UpdatedAt,
			&suggestion.ExpiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan habit suggestion: %w", err)
		}

		if recurrenceEndDate.Valid {
			suggestion.RecurrenceEndDate = &recurrenceEndDate.Time
		}

		suggestions = append(suggestions, suggestion)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating habit suggestions: %w", err)
	}

	return suggestions, nil
}

// GetHabitSuggestionByID retrieves a habit suggestion by ID
func (r *SQLRepository) GetHabitSuggestionByID(ctx context.Context, id string) (*models.HabitSuggestion, error) {
	query := `
		SELECT id, user_id, event_frequency_id, title, description, location,
			   duration_minutes, time_of_day, day_of_week, status,
			   recurrence_end_date, created_at, updated_at, expires_at
		FROM habit_suggestions
		WHERE id = $1
	`

	suggestion := &models.HabitSuggestion{}
	var recurrenceEndDate sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&suggestion.ID,
		&suggestion.UserID,
		&suggestion.EventFrequencyID,
		&suggestion.Title,
		&suggestion.Description,
		&suggestion.Location,
		&suggestion.DurationMinutes,
		&suggestion.TimeOfDay,
		&suggestion.DayOfWeek,
		&suggestion.Status,
		&recurrenceEndDate,
		&suggestion.CreatedAt,
		&suggestion.UpdatedAt,
		&suggestion.ExpiresAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("habit suggestion not found")
		}
		return nil, fmt.Errorf("failed to get habit suggestion: %w", err)
	}

	if recurrenceEndDate.Valid {
		suggestion.RecurrenceEndDate = &recurrenceEndDate.Time
	}

	return suggestion, nil
}

// UpdateHabitSuggestionStatus updates the status of a habit suggestion
func (r *SQLRepository) UpdateHabitSuggestionStatus(ctx context.Context, id, status string, recurrenceEndDate *time.Time) error {
	query := `
		UPDATE habit_suggestions
		SET status = $1, recurrence_end_date = $2, updated_at = $3
		WHERE id = $4
	`

	_, err := r.db.ExecContext(ctx, query, status, recurrenceEndDate, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update habit suggestion status: %w", err)
	}
	return nil
}

// ===== Recurring Event Repository Implementation =====

// CreateRecurringEvent inserts a new recurring event
func (r *SQLRepository) CreateRecurringEvent(ctx context.Context, recurring *models.RecurringEvent) error {
	query := `
		INSERT INTO recurring_events (
			id, user_id, habit_suggestion_id, title, description, location,
			duration_minutes, time_of_day, day_of_week, start_date, end_date,
			is_active, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err := r.db.ExecContext(ctx, query,
		recurring.ID,
		recurring.UserID,
		recurring.HabitSuggestionID,
		recurring.Title,
		recurring.Description,
		recurring.Location,
		recurring.DurationMinutes,
		recurring.TimeOfDay,
		recurring.DayOfWeek,
		recurring.StartDate,
		recurring.EndDate,
		recurring.IsActive,
		recurring.CreatedAt,
		recurring.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create recurring event: %w", err)
	}
	return nil
}

// GetActiveRecurringEvents retrieves all active recurring events for a user
func (r *SQLRepository) GetActiveRecurringEvents(ctx context.Context, userID string) ([]*models.RecurringEvent, error) {
	query := `
		SELECT id, user_id, habit_suggestion_id, title, description, location,
			   duration_minutes, time_of_day, day_of_week, start_date, end_date,
			   is_active, created_at, updated_at
		FROM recurring_events
		WHERE user_id = $1 AND is_active = TRUE AND end_date > NOW()
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active recurring events: %w", err)
	}
	defer rows.Close()

	var events []*models.RecurringEvent
	for rows.Next() {
		event := &models.RecurringEvent{}
		var habitSuggestionID sql.NullString

		err := rows.Scan(
			&event.ID,
			&event.UserID,
			&habitSuggestionID,
			&event.Title,
			&event.Description,
			&event.Location,
			&event.DurationMinutes,
			&event.TimeOfDay,
			&event.DayOfWeek,
			&event.StartDate,
			&event.EndDate,
			&event.IsActive,
			&event.CreatedAt,
			&event.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan recurring event: %w", err)
		}

		if habitSuggestionID.Valid {
			event.HabitSuggestionID = &habitSuggestionID.String
		}

		events = append(events, event)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating recurring events: %w", err)
	}

	return events, nil
}

// GetRecurringEventByID retrieves a recurring event by ID
func (r *SQLRepository) GetRecurringEventByID(ctx context.Context, id string) (*models.RecurringEvent, error) {
	query := `
		SELECT id, user_id, habit_suggestion_id, title, description, location,
			   duration_minutes, time_of_day, day_of_week, start_date, end_date,
			   is_active, created_at, updated_at
		FROM recurring_events
		WHERE id = $1
	`

	event := &models.RecurringEvent{}
	var habitSuggestionID sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&event.ID,
		&event.UserID,
		&habitSuggestionID,
		&event.Title,
		&event.Description,
		&event.Location,
		&event.DurationMinutes,
		&event.TimeOfDay,
		&event.DayOfWeek,
		&event.StartDate,
		&event.EndDate,
		&event.IsActive,
		&event.CreatedAt,
		&event.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("recurring event not found")
		}
		return nil, fmt.Errorf("failed to get recurring event: %w", err)
	}

	if habitSuggestionID.Valid {
		event.HabitSuggestionID = &habitSuggestionID.String
	}

	return event, nil
}

// DeactivateRecurringEvent deactivates a recurring event
func (r *SQLRepository) DeactivateRecurringEvent(ctx context.Context, id string) error {
	query := `
		UPDATE recurring_events
		SET is_active = FALSE, updated_at = $1
		WHERE id = $2
	`

	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to deactivate recurring event: %w", err)
	}
	return nil
}

