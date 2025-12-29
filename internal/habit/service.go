package habit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
)

const (
	// DefaultSuggestionThreshold is the minimum occurrence count to trigger a habit suggestion
	DefaultSuggestionThreshold = 3
	// DefaultRecurrenceYears is the number of years to create recurring events for
	DefaultRecurrenceYears = 5
	// TimeToleranceMinutes is the tolerance for matching event times (within 30 minutes)
	TimeToleranceMinutes = 30
)

// Service represents the Habit Tracking Service
type Service struct {
	config *config.Config
	logger *logger.Logger
	repo   Repository
}

// NewService creates a new Habit Tracking Service
func NewService(cfg *config.Config, log *logger.Logger, repo Repository) *Service {
	return &Service{
		config: cfg,
		logger: log,
		repo:   repo,
	}
}

// RegisterRoutes registers habit tracking routes
func (s *Service) RegisterRoutes(router *mux.Router) {
	habitRouter := router.PathPrefix("/habit").Subrouter()

	// Get pending habit suggestions for a user
	habitRouter.HandleFunc("/suggestions", s.handleGetSuggestions).Methods("GET")
	// Accept a habit suggestion
	habitRouter.HandleFunc("/suggestions/{id}/accept", s.handleAcceptSuggestion).Methods("POST")
	// Reject a habit suggestion
	habitRouter.HandleFunc("/suggestions/{id}/reject", s.handleRejectSuggestion).Methods("POST")
	// Get active recurring events
	habitRouter.HandleFunc("/recurring", s.handleGetRecurringEvents).Methods("GET")
	// Deactivate a recurring event
	habitRouter.HandleFunc("/recurring/{id}/deactivate", s.handleDeactivateRecurringEvent).Methods("POST")
	// Get event frequencies (for debugging/admin)
	habitRouter.HandleFunc("/frequencies", s.handleGetFrequencies).Methods("GET")
}

// TrackEventCreation analyzes a newly created event and updates frequency patterns
// This method should be called after an event is successfully created
func (s *Service) TrackEventCreation(ctx context.Context, event *models.Event) error {
	s.logger.Info("Tracking event creation for habit detection",
		"user_id", event.UserID,
		"title", event.Title,
		"start_time", event.StartTime)

	// Calculate event pattern characteristics
	durationMinutes := int(event.EndTime.Sub(event.StartTime).Minutes())
	timeOfDay := event.StartTime.Hour()*60 + event.StartTime.Minute()
	dayOfWeek := int(event.StartTime.Weekday())

	// Try to find existing frequency record
	existing, err := s.repo.GetEventFrequencyByPattern(
		ctx, event.UserID, event.Title, durationMinutes, timeOfDay, dayOfWeek,
	)
	if err != nil {
		return fmt.Errorf("failed to get event frequency: %w", err)
	}

	now := time.Now()

	if existing != nil {
		// Update existing frequency record
		existing.OccurrenceCount++
		existing.OccurrenceTimestamps = append(existing.OccurrenceTimestamps, event.StartTime)
		// Keep only the last 10 timestamps
		if len(existing.OccurrenceTimestamps) > 10 {
			existing.OccurrenceTimestamps = existing.OccurrenceTimestamps[len(existing.OccurrenceTimestamps)-10:]
		}

		if err := s.repo.UpdateEventFrequency(ctx, existing); err != nil {
			return fmt.Errorf("failed to update event frequency: %w", err)
		}

		s.logger.Info("Updated event frequency",
			"frequency_id", existing.ID,
			"occurrence_count", existing.OccurrenceCount,
			"threshold", existing.SuggestionThreshold)

		// Check if we should create a suggestion
		if existing.OccurrenceCount >= existing.SuggestionThreshold && !existing.SuggestionShown && !existing.HabitAccepted {
			if err := s.createSuggestionFromFrequency(ctx, existing); err != nil {
				s.logger.Error("Failed to create habit suggestion", "error", err)
			}
		}
	} else {
		// Create new frequency record
		freq := &models.EventFrequency{
			ID:                   uuid.New().String(),
			UserID:               event.UserID,
			Title:                event.Title,
			Description:          event.Description,
			Location:             event.Location,
			DurationMinutes:      durationMinutes,
			TimeOfDay:            timeOfDay,
			DayOfWeek:            dayOfWeek,
			OccurrenceCount:      1,
			SuggestionThreshold:  DefaultSuggestionThreshold,
			SuggestionShown:      false,
			HabitAccepted:        false,
			OccurrenceTimestamps: []time.Time{event.StartTime},
			CreatedAt:            now,
			UpdatedAt:            now,
		}

		if err := s.repo.UpsertEventFrequency(ctx, freq); err != nil {
			return fmt.Errorf("failed to create event frequency: %w", err)
		}

		s.logger.Info("Created new event frequency record",
			"frequency_id", freq.ID,
			"title", freq.Title)
	}

	return nil
}

// createSuggestionFromFrequency creates a habit suggestion from a frequency record
func (s *Service) createSuggestionFromFrequency(ctx context.Context, freq *models.EventFrequency) error {
	now := time.Now()

	suggestion := &models.HabitSuggestion{
		ID:               uuid.New().String(),
		UserID:           freq.UserID,
		EventFrequencyID: freq.ID,
		Title:            freq.Title,
		Description:      freq.Description,
		Location:         freq.Location,
		DurationMinutes:  freq.DurationMinutes,
		TimeOfDay:        freq.TimeOfDay,
		DayOfWeek:        freq.DayOfWeek,
		Status:           "pending",
		CreatedAt:        now,
		UpdatedAt:        now,
		ExpiresAt:        now.Add(7 * 24 * time.Hour), // Expires in 7 days
	}

	if err := s.repo.CreateHabitSuggestion(ctx, suggestion); err != nil {
		return fmt.Errorf("failed to create habit suggestion: %w", err)
	}

	// Mark frequency as suggestion shown
	freq.SuggestionShown = true
	if err := s.repo.UpdateEventFrequency(ctx, freq); err != nil {
		return fmt.Errorf("failed to update frequency suggestion_shown: %w", err)
	}

	s.logger.Info("Created habit suggestion",
		"suggestion_id", suggestion.ID,
		"user_id", suggestion.UserID,
		"title", suggestion.Title,
		"occurrence_count", freq.OccurrenceCount)

	return nil
}

// GetPendingSuggestions returns all pending habit suggestions for a user
func (s *Service) GetPendingSuggestions(ctx context.Context, userID string) ([]*models.HabitSuggestion, error) {
	return s.repo.GetPendingHabitSuggestions(ctx, userID)
}

// AcceptSuggestion accepts a habit suggestion and creates recurring events for 5 years
func (s *Service) AcceptSuggestion(ctx context.Context, suggestionID string) (*models.RecurringEvent, error) {
	suggestion, err := s.repo.GetHabitSuggestionByID(ctx, suggestionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get suggestion: %w", err)
	}

	if suggestion.Status != "pending" {
		return nil, fmt.Errorf("suggestion is not pending, current status: %s", suggestion.Status)
	}

	now := time.Now()
	endDate := now.AddDate(DefaultRecurrenceYears, 0, 0) // 5 years from now

	// Create recurring event
	recurring := &models.RecurringEvent{
		ID:                uuid.New().String(),
		UserID:            suggestion.UserID,
		HabitSuggestionID: &suggestion.ID,
		Title:             suggestion.Title,
		Description:       suggestion.Description,
		Location:          suggestion.Location,
		DurationMinutes:   suggestion.DurationMinutes,
		TimeOfDay:         suggestion.TimeOfDay,
		DayOfWeek:         suggestion.DayOfWeek,
		StartDate:         now,
		EndDate:           endDate,
		IsActive:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.repo.CreateRecurringEvent(ctx, recurring); err != nil {
		return nil, fmt.Errorf("failed to create recurring event: %w", err)
	}

	// Update suggestion status
	if err := s.repo.UpdateHabitSuggestionStatus(ctx, suggestionID, "accepted", &endDate); err != nil {
		return nil, fmt.Errorf("failed to update suggestion status: %w", err)
	}

	// Update frequency to mark habit as accepted
	freq, err := s.repo.GetEventFrequencyByPattern(
		ctx, suggestion.UserID, suggestion.Title,
		suggestion.DurationMinutes, suggestion.TimeOfDay, suggestion.DayOfWeek,
	)
	if err == nil && freq != nil {
		freq.HabitAccepted = true
		if err := s.repo.UpdateEventFrequency(ctx, freq); err != nil {
			s.logger.Warn("Failed to update frequency habit_accepted", "error", err)
		}
	}

	s.logger.Info("Accepted habit suggestion and created recurring event",
		"suggestion_id", suggestionID,
		"recurring_id", recurring.ID,
		"end_date", endDate)

	return recurring, nil
}

// RejectSuggestion rejects a habit suggestion
func (s *Service) RejectSuggestion(ctx context.Context, suggestionID string) error {
	suggestion, err := s.repo.GetHabitSuggestionByID(ctx, suggestionID)
	if err != nil {
		return fmt.Errorf("failed to get suggestion: %w", err)
	}

	if suggestion.Status != "pending" {
		return fmt.Errorf("suggestion is not pending, current status: %s", suggestion.Status)
	}

	if err := s.repo.UpdateHabitSuggestionStatus(ctx, suggestionID, "rejected", nil); err != nil {
		return fmt.Errorf("failed to update suggestion status: %w", err)
	}

	s.logger.Info("Rejected habit suggestion", "suggestion_id", suggestionID)
	return nil
}

// GetRecurringEventsForTimeRange returns recurring events that should occur in a given time range
// This is used by the calendar service to include habit events in event listings
func (s *Service) GetRecurringEventsForTimeRange(ctx context.Context, userID string, startTime, endTime time.Time) ([]*models.Event, error) {
	recurring, err := s.repo.GetActiveRecurringEvents(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active recurring events: %w", err)
	}

	var events []*models.Event

	for _, r := range recurring {
		// Generate events for the given time range
		generatedEvents := s.generateEventsFromRecurring(r, startTime, endTime)
		events = append(events, generatedEvents...)
	}

	return events, nil
}

// generateEventsFromRecurring generates individual events from a recurring event template
func (s *Service) generateEventsFromRecurring(recurring *models.RecurringEvent, startTime, endTime time.Time) []*models.Event {
	var events []*models.Event

	// Start from the beginning of the week containing startTime
	current := startTime.Truncate(24 * time.Hour)

	// Find the first day matching the day_of_week
	for int(current.Weekday()) != recurring.DayOfWeek {
		current = current.Add(24 * time.Hour)
	}

	// Generate events weekly until endTime
	for current.Before(endTime) && current.Before(recurring.EndDate) {
		if current.After(recurring.StartDate) || current.Equal(recurring.StartDate) {
			// Calculate event start time for this occurrence
			eventStart := time.Date(
				current.Year(), current.Month(), current.Day(),
				recurring.TimeOfDay/60, recurring.TimeOfDay%60, 0, 0,
				current.Location(),
			)
			eventEnd := eventStart.Add(time.Duration(recurring.DurationMinutes) * time.Minute)

			// Only include if within the requested range
			if eventStart.After(startTime) || eventStart.Equal(startTime) {
				event := &models.Event{
					ID:          fmt.Sprintf("recurring-%s-%s", recurring.ID, eventStart.Format("2006-01-02")),
					UserID:      recurring.UserID,
					Title:       recurring.Title,
					Description: recurring.Description,
					StartTime:   eventStart,
					EndTime:     eventEnd,
					Location:    recurring.Location,
				}
				events = append(events, event)
			}
		}

		// Move to next week
		current = current.Add(7 * 24 * time.Hour)
	}

	return events
}

// ===== HTTP Handlers =====

// handleGetSuggestions returns pending habit suggestions for a user
func (s *Service) handleGetSuggestions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "user_id required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	suggestions, err := s.GetPendingSuggestions(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get habit suggestions", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to get suggestions"})
		return
	}

	// Convert to response format with readable time
	response := make([]map[string]interface{}, len(suggestions))
	for i, suggestion := range suggestions {
		response[i] = map[string]interface{}{
			"id":               suggestion.ID,
			"user_id":          suggestion.UserID,
			"title":            suggestion.Title,
			"description":      suggestion.Description,
			"location":         suggestion.Location,
			"duration_minutes": suggestion.DurationMinutes,
			"time_of_day":      formatTimeOfDay(suggestion.TimeOfDay),
			"day_of_week":      formatDayOfWeek(suggestion.DayOfWeek),
			"status":           suggestion.Status,
			"created_at":       suggestion.CreatedAt,
			"expires_at":       suggestion.ExpiresAt,
			"message":          fmt.Sprintf("You've scheduled '%s' multiple times on %s at %s. Would you like to make this a recurring event for the next 5 years?", suggestion.Title, formatDayOfWeek(suggestion.DayOfWeek), formatTimeOfDay(suggestion.TimeOfDay)),
		}
	}

	json.NewEncoder(w).Encode(response)
}

// handleAcceptSuggestion accepts a habit suggestion
func (s *Service) handleAcceptSuggestion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	suggestionID := mux.Vars(r)["id"]

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	recurring, err := s.AcceptSuggestion(ctx, suggestionID)
	if err != nil {
		s.logger.Error("Failed to accept habit suggestion", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":          true,
		"message":          fmt.Sprintf("Habit accepted! '%s' will be scheduled every %s at %s until %s", recurring.Title, formatDayOfWeek(recurring.DayOfWeek), formatTimeOfDay(recurring.TimeOfDay), recurring.EndDate.Format("January 2, 2006")),
		"recurring_event":  recurring,
	})
}

// handleRejectSuggestion rejects a habit suggestion
func (s *Service) handleRejectSuggestion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	suggestionID := mux.Vars(r)["id"]

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := s.RejectSuggestion(ctx, suggestionID); err != nil {
		s.logger.Error("Failed to reject habit suggestion", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Habit suggestion rejected",
	})
}

// handleGetRecurringEvents returns active recurring events for a user
func (s *Service) handleGetRecurringEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "user_id required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	recurring, err := s.repo.GetActiveRecurringEvents(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get recurring events", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to get recurring events"})
		return
	}

	// Convert to response format
	response := make([]map[string]interface{}, len(recurring))
	for i, r := range recurring {
		response[i] = map[string]interface{}{
			"id":               r.ID,
			"user_id":          r.UserID,
			"title":            r.Title,
			"description":      r.Description,
			"location":         r.Location,
			"duration_minutes": r.DurationMinutes,
			"time_of_day":      formatTimeOfDay(r.TimeOfDay),
			"day_of_week":      formatDayOfWeek(r.DayOfWeek),
			"start_date":       r.StartDate,
			"end_date":         r.EndDate,
			"is_active":        r.IsActive,
			"created_at":       r.CreatedAt,
		}
	}

	json.NewEncoder(w).Encode(response)
}

// handleDeactivateRecurringEvent deactivates a recurring event
func (s *Service) handleDeactivateRecurringEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	recurringID := mux.Vars(r)["id"]

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := s.repo.DeactivateRecurringEvent(ctx, recurringID); err != nil {
		s.logger.Error("Failed to deactivate recurring event", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Recurring event deactivated",
	})
}

// handleGetFrequencies returns event frequencies for debugging/admin
func (s *Service) handleGetFrequencies(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "user_id required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Get all frequencies above threshold 1 (to show patterns)
	frequencies, err := s.repo.GetEventFrequenciesAboveThreshold(ctx, userID, 1)
	if err != nil {
		s.logger.Error("Failed to get event frequencies", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to get frequencies"})
		return
	}

	// Convert to response format
	response := make([]map[string]interface{}, len(frequencies))
	for i, freq := range frequencies {
		response[i] = map[string]interface{}{
			"id":                   freq.ID,
			"user_id":              freq.UserID,
			"title":                freq.Title,
			"description":          freq.Description,
			"location":             freq.Location,
			"duration_minutes":     freq.DurationMinutes,
			"time_of_day":          formatTimeOfDay(freq.TimeOfDay),
			"day_of_week":          formatDayOfWeek(freq.DayOfWeek),
			"occurrence_count":     freq.OccurrenceCount,
			"suggestion_threshold": freq.SuggestionThreshold,
			"suggestion_shown":     freq.SuggestionShown,
			"habit_accepted":       freq.HabitAccepted,
			"created_at":           freq.CreatedAt,
		}
	}

	json.NewEncoder(w).Encode(response)
}

// Helper functions

// formatTimeOfDay converts minutes from midnight to a readable time string
func formatTimeOfDay(minutesFromMidnight int) string {
	hours := minutesFromMidnight / 60
	minutes := minutesFromMidnight % 60
	period := "AM"
	if hours >= 12 {
		period = "PM"
		if hours > 12 {
			hours -= 12
		}
	}
	if hours == 0 {
		hours = 12
	}
	return fmt.Sprintf("%d:%02d %s", hours, minutes, period)
}

// formatDayOfWeek converts day number to day name
func formatDayOfWeek(day int) string {
	days := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	if day >= 0 && day < len(days) {
		return days[day]
	}
	return "Unknown"
}

