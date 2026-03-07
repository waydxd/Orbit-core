package habit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/waydxd/Orbit-core/internal/shared/database"
	"github.com/waydxd/Orbit-core/internal/shared/database/db"
	"github.com/waydxd/Orbit-core/internal/shared/models"
)

// Repository defines database operations for habit tracking
type Repository interface {
	// UpsertEventFrequency GetEventFrequencyByPattern
	// GetEventFrequenciesAboveThreshold UpdateEventFrequency
	// Event frequency operations
	UpsertEventFrequency(ctx context.Context, freq *models.EventFrequency) error
	GetEventFrequencyByPattern(ctx context.Context, userID, title string, durationMinutes, timeOfDay, dayOfWeek int) (*models.EventFrequency, error)
	GetEventFrequenciesAboveThreshold(ctx context.Context, userID string, threshold int) ([]*models.EventFrequency, error)
	UpdateEventFrequency(ctx context.Context, freq *models.EventFrequency) error

	// CreateHabitSuggestion GetPendingHabitSuggestions
	// GetHabitSuggestionByID UpdateHabitSuggestionStatus
	// Habit suggestion operations
	CreateHabitSuggestion(ctx context.Context, suggestion *models.HabitSuggestion) error
	GetPendingHabitSuggestions(ctx context.Context, userID string) ([]*models.HabitSuggestion, error)
	GetHabitSuggestionByID(ctx context.Context, id string) (*models.HabitSuggestion, error)
	UpdateHabitSuggestionStatus(ctx context.Context, id, status string, recurrenceEndDate *time.Time) error

	// CreateRecurringEvent GetActiveRecurringEvents DeactivateRecurringEvent
	// Recurring event operations (now using Event model with IsRecurring flag)
	CreateRecurringEvent(ctx context.Context, event *models.Event) error
	GetActiveRecurringEvents(ctx context.Context, userID string) ([]*models.Event, error)
	DeactivateRecurringEvent(ctx context.Context, eventID string) error
}

// SQLRepository implements Repository using PostgreSQL
type SQLRepository struct {
	queries *db.Queries
	pool    *database.DB
}

// NewSQLRepository creates a new SQL habit repository
func NewSQLRepository(pool *database.DB) Repository {
	return &SQLRepository{
		queries: db.New(pool.Pool),
		pool:    pool,
	}
}

// ===== Event Frequency Repository Implementation =====

// UpsertEventFrequency inserts or updates an event frequency record
func (r *SQLRepository) UpsertEventFrequency(ctx context.Context, freq *models.EventFrequency) error {
	// Convert occurrence timestamps to JSON
	timestampsJSON, err := json.Marshal(freq.OccurrenceTimestamps)
	if err != nil {
		return fmt.Errorf("failed to marshal occurrence timestamps: %w", err)
	}

	params := db.UpsertEventFrequencyParams{
		ID:                   database.StringToUUID(freq.ID),
		UserID:               database.StringToUUID(freq.UserID),
		Title:                freq.Title,
		Description:          database.StringToText(freq.Description),
		Location:             database.StringToText(freq.Location),
		DurationMinutes:      database.IntToInt32(freq.DurationMinutes),
		TimeOfDay:            database.IntToInt32(freq.TimeOfDay),
		DayOfWeek:            database.IntToInt32(freq.DayOfWeek),
		OccurrenceCount:      database.IntToInt32(freq.OccurrenceCount),
		SuggestionThreshold:  database.IntToInt32(freq.SuggestionThreshold),
		SuggestionShown:      freq.SuggestionShown,
		HabitAccepted:        freq.HabitAccepted,
		OccurrenceTimestamps: timestampsJSON,
		CreatedAt:            database.TimeToTimestamptz(freq.CreatedAt),
		UpdatedAt:            database.TimeToTimestamptz(freq.UpdatedAt),
	}

	row, err := r.queries.UpsertEventFrequency(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to upsert event frequency: %w", err)
	}

	freq.ID = database.UUIDToString(row.ID)
	freq.OccurrenceCount = int(row.OccurrenceCount)
	return nil
}

// GetEventFrequencyByPattern retrieves an event frequency by its pattern
func (r *SQLRepository) GetEventFrequencyByPattern(ctx context.Context, userID, title string, durationMinutes, timeOfDay, dayOfWeek int) (*models.EventFrequency, error) {
	params := db.GetEventFrequencyByPatternParams{
		UserID:          database.StringToUUID(userID),
		Title:           title,
		DurationMinutes: database.IntToInt32(durationMinutes),
		TimeOfDay:       database.IntToInt32(timeOfDay),
		DayOfWeek:       database.IntToInt32(dayOfWeek),
	}

	row, err := r.queries.GetEventFrequencyByPattern(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get event frequency: %w", err)
	}

	freq := &models.EventFrequency{
		ID:                  database.UUIDToString(row.ID),
		UserID:              database.UUIDToString(row.UserID),
		Title:               row.Title,
		Description:         database.TextToString(row.Description),
		Location:            database.TextToString(row.Location),
		DurationMinutes:     int(row.DurationMinutes),
		TimeOfDay:           int(row.TimeOfDay),
		DayOfWeek:           0,
		OccurrenceCount:     int(row.OccurrenceCount),
		SuggestionThreshold: int(row.SuggestionThreshold),
		SuggestionShown:     row.SuggestionShown,
		HabitAccepted:       row.HabitAccepted,
		CreatedAt:           database.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:           database.TimestamptzToTime(row.UpdatedAt),
	}

	if err := json.Unmarshal(row.OccurrenceTimestamps, &freq.OccurrenceTimestamps); err != nil {
		return nil, fmt.Errorf("failed to unmarshal occurrence timestamps: %w", err)
	}

	freq.DayOfWeek = int(row.DayOfWeek)

	return freq, nil
}

// GetEventFrequenciesAboveThreshold retrieves event frequencies that meet or exceed the suggestion threshold
func (r *SQLRepository) GetEventFrequenciesAboveThreshold(ctx context.Context, userID string, threshold int) ([]*models.EventFrequency, error) {
	params := db.GetEventFrequenciesAboveThresholdParams{
		UserID:          database.StringToUUID(userID),
		OccurrenceCount: database.IntToInt32(threshold),
	}

	rows, err := r.queries.GetEventFrequenciesAboveThreshold(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get event frequencies: %w", err)
	}

	frequencies := make([]*models.EventFrequency, 0, len(rows))
	for _, row := range rows {
		freq := &models.EventFrequency{
			ID:                  database.UUIDToString(row.ID),
			UserID:              database.UUIDToString(row.UserID),
			Title:               row.Title,
			Description:         database.TextToString(row.Description),
			Location:            database.TextToString(row.Location),
			DurationMinutes:     int(row.DurationMinutes),
			TimeOfDay:           int(row.TimeOfDay),
			DayOfWeek:           0,
			OccurrenceCount:     int(row.OccurrenceCount),
			SuggestionThreshold: int(row.SuggestionThreshold),
			SuggestionShown:     row.SuggestionShown,
			HabitAccepted:       row.HabitAccepted,
			CreatedAt:           database.TimestamptzToTime(row.CreatedAt),
			UpdatedAt:           database.TimestamptzToTime(row.UpdatedAt),
		}

		if err := json.Unmarshal(row.OccurrenceTimestamps, &freq.OccurrenceTimestamps); err != nil {
			return nil, fmt.Errorf("failed to unmarshal occurrence timestamps: %w", err)
		}

		freq.DayOfWeek = int(row.DayOfWeek)

		frequencies = append(frequencies, freq)
	}

	return frequencies, nil
}

// UpdateEventFrequency updates an existing event frequency record
func (r *SQLRepository) UpdateEventFrequency(ctx context.Context, freq *models.EventFrequency) error {
	timestampsJSON, err := json.Marshal(freq.OccurrenceTimestamps)
	if err != nil {
		return fmt.Errorf("failed to marshal occurrence timestamps: %w", err)
	}

	params := db.UpdateEventFrequencyParams{
		OccurrenceCount:      database.IntToInt32(freq.OccurrenceCount),
		SuggestionShown:      freq.SuggestionShown,
		HabitAccepted:        freq.HabitAccepted,
		OccurrenceTimestamps: timestampsJSON,
		UpdatedAt:            database.TimeToTimestamptz(time.Now()),
		ID:                   database.StringToUUID(freq.ID),
	}

	err = r.queries.UpdateEventFrequency(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to update event frequency: %w", err)
	}
	return nil
}

// ===== Habit Suggestion Repository Implementation =====

// CreateHabitSuggestion inserts a new habit suggestion
func (r *SQLRepository) CreateHabitSuggestion(ctx context.Context, suggestion *models.HabitSuggestion) error {
	params := db.CreateHabitSuggestionParams{
		ID:               database.StringToUUID(suggestion.ID),
		UserID:           database.StringToUUID(suggestion.UserID),
		EventFrequencyID: database.StringToUUID(suggestion.EventFrequencyID),
		Title:            suggestion.Title,
		Description:      database.StringToText(suggestion.Description),
		Location:         database.StringToText(suggestion.Location),
		DurationMinutes:  database.IntToInt32(suggestion.DurationMinutes),
		TimeOfDay:        database.IntToInt32(suggestion.TimeOfDay),
		DayOfWeek:        database.IntToInt32(suggestion.DayOfWeek),
		Status:           database.StringToText(suggestion.Status),
		CreatedAt:        database.TimeToTimestamptz(suggestion.CreatedAt),
		UpdatedAt:        database.TimeToTimestamptz(suggestion.UpdatedAt),
		ExpiresAt:        database.TimeToTimestamptz(suggestion.ExpiresAt),
	}

	err := r.queries.CreateHabitSuggestion(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to create habit suggestion: %w", err)
	}
	return nil
}

// GetPendingHabitSuggestions retrieves all pending habit suggestions for a user
func (r *SQLRepository) GetPendingHabitSuggestions(ctx context.Context, userID string) ([]*models.HabitSuggestion, error) {
	rows, err := r.queries.GetPendingHabitSuggestions(ctx, database.StringToUUID(userID))
	if err != nil {
		return nil, fmt.Errorf("failed to get pending habit suggestions: %w", err)
	}

	suggestions := make([]*models.HabitSuggestion, 0, len(rows))
	for _, row := range rows {
		suggestion := &models.HabitSuggestion{
			ID:               database.UUIDToString(row.ID),
			UserID:           database.UUIDToString(row.UserID),
			EventFrequencyID: database.UUIDToString(row.EventFrequencyID),
			Title:            row.Title,
			Description:      database.TextToString(row.Description),
			Location:         database.TextToString(row.Location),
			DurationMinutes:  int(row.DurationMinutes),
			TimeOfDay:        int(row.TimeOfDay),
			DayOfWeek:        int(row.DayOfWeek),
			Status:           database.TextToString(row.Status),
			CreatedAt:        database.TimestamptzToTime(row.CreatedAt),
			UpdatedAt:        database.TimestamptzToTime(row.UpdatedAt),
			ExpiresAt:        database.TimestamptzToTime(row.ExpiresAt),
		}

		if row.RecurrenceEndDate.Valid {
			endTime := database.TimestamptzToTime(row.RecurrenceEndDate)
			suggestion.RecurrenceEndDate = &endTime
		}

		suggestions = append(suggestions, suggestion)
	}

	return suggestions, nil
}

// GetHabitSuggestionByID retrieves a habit suggestion by ID
func (r *SQLRepository) GetHabitSuggestionByID(ctx context.Context, id string) (*models.HabitSuggestion, error) {
	row, err := r.queries.GetHabitSuggestionByID(ctx, database.StringToUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("habit suggestion not found")
		}
		return nil, fmt.Errorf("failed to get habit suggestion: %w", err)
	}

	suggestion := &models.HabitSuggestion{
		ID:               database.UUIDToString(row.ID),
		UserID:           database.UUIDToString(row.UserID),
		EventFrequencyID: database.UUIDToString(row.EventFrequencyID),
		Title:            row.Title,
		Description:      database.TextToString(row.Description),
		Location:         database.TextToString(row.Location),
		DurationMinutes:  int(row.DurationMinutes),
		TimeOfDay:        int(row.TimeOfDay),
		DayOfWeek:        int(row.DayOfWeek),
		Status:           database.TextToString(row.Status),
		CreatedAt:        database.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:        database.TimestamptzToTime(row.UpdatedAt),
		ExpiresAt:        database.TimestamptzToTime(row.ExpiresAt),
	}

	if row.RecurrenceEndDate.Valid {
		endTime := database.TimestamptzToTime(row.RecurrenceEndDate)
		suggestion.RecurrenceEndDate = &endTime
	}

	return suggestion, nil
}

// UpdateHabitSuggestionStatus updates the status of a habit suggestion
func (r *SQLRepository) UpdateHabitSuggestionStatus(ctx context.Context, id, status string, recurrenceEndDate *time.Time) error {
	var recurrenceParam pgtype.Timestamptz
	if recurrenceEndDate != nil {
		recurrenceParam = database.TimeToTimestamptz(*recurrenceEndDate)
	} else {
		recurrenceParam = pgtype.Timestamptz{Valid: false}
	}

	params := db.UpdateHabitSuggestionStatusParams{
		Status:            database.StringToText(status),
		RecurrenceEndDate: recurrenceParam,
		UpdatedAt:         database.TimeToTimestamptz(time.Now()),
		ID:                database.StringToUUID(id),
	}

	err := r.queries.UpdateHabitSuggestionStatus(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to update habit suggestion status: %w", err)
	}
	return nil
}

// ===== Recurring Event Repository Implementation =====

// CreateRecurringEvent creates a recurring event (stored as an Event with IsRecurring flag)
func (r *SQLRepository) CreateRecurringEvent(ctx context.Context, event *models.Event) error {
	// Ensure IsRecurring is set
	event.IsRecurring = true

	params := db.CreateRecurringEventParams{
		ID:                  database.StringToUUID(event.ID),
		UserID:              database.StringToUUID(event.UserID),
		Title:               event.Title,
		Description:         database.StringToText(event.Description),
		Location:            database.StringToText(event.Location),
		StartTime:           database.TimeToTimestamptz(event.StartTime),
		EndTime:             database.TimeToTimestamptz(event.EndTime),
		IsRecurring:         pgtype.Bool{Bool: event.IsRecurring, Valid: true},
		RecurrenceRule:      database.StringToText(event.RecurrenceRule),
		RecurrenceException: database.StringToText(event.RecurrenceException),
		CreatedAt:           database.TimeToTimestamptz(event.CreatedAt),
		UpdatedAt:           database.TimeToTimestamptz(event.UpdatedAt),
	}

	err := r.queries.CreateRecurringEvent(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to create recurring event: %w", err)
	}
	return nil
}

// GetActiveRecurringEvents retrieves all active recurring events for a user
func (r *SQLRepository) GetActiveRecurringEvents(ctx context.Context, userID string) ([]*models.Event, error) {
	rows, err := r.queries.GetActiveRecurringEvents(ctx, database.StringToUUID(userID))
	if err != nil {
		return nil, fmt.Errorf("failed to get active recurring events: %w", err)
	}

	events := make([]*models.Event, 0, len(rows))
	for _, row := range rows {
		event := &models.Event{
			ID:                  database.UUIDToString(row.ID),
			UserID:              database.UUIDToString(row.UserID),
			Title:               row.Title,
			Description:         database.TextToString(row.Description),
			Location:            database.TextToString(row.Location),
			StartTime:           database.TimestamptzToTime(row.StartTime),
			EndTime:             database.TimestamptzToTime(row.EndTime),
			IsRecurring:         row.IsRecurring.Bool,
			RecurrenceRule:      database.TextToString(row.RecurrenceRule),
			RecurrenceException: database.TextToString(row.RecurrenceException),
			CreatedAt:           database.TimestamptzToTime(row.CreatedAt),
			UpdatedAt:           database.TimestamptzToTime(row.UpdatedAt),
		}

		events = append(events, event)
	}

	return events, nil
}

// DeactivateRecurringEvent deactivates a recurring event by setting is_recurring to FALSE
func (r *SQLRepository) DeactivateRecurringEvent(ctx context.Context, eventID string) error {
	params := db.DeactivateRecurringEventParams{
		UpdatedAt: database.TimeToTimestamptz(time.Now()),
		ID:        database.StringToUUID(eventID),
	}

	err := r.queries.DeactivateRecurringEvent(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to deactivate recurring event: %w", err)
	}
	return nil
}
