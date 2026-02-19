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
	"github.com/waydxd/Orbit-core/pkg/middleware"
)

const (
	// DefaultSuggestionThreshold is the minimum occurrence count to trigger a habit suggestion
	DefaultSuggestionThreshold = 3
	// DefaultRecurrenceYears is the number of years to create recurring events for
	DefaultRecurrenceYears = 5
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

	// Get pending habit suggestions for the authenticated user
	habitRouter.HandleFunc("/suggestions", s.handleGetSuggestions).Methods("GET")
	// Accept a habit suggestion
	habitRouter.HandleFunc("/suggestions/{id}/accept", s.handleAcceptSuggestion).Methods("POST")
	// Reject a habit suggestion
	habitRouter.HandleFunc("/suggestions/{id}/reject", s.handleRejectSuggestion).Methods("POST")
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

// AcceptSuggestion accepts a habit suggestion and creates a recurring event
func (s *Service) AcceptSuggestion(ctx context.Context, suggestionID string) (*models.Event, error) {
	suggestion, err := s.repo.GetHabitSuggestionByID(ctx, suggestionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get suggestion: %w", err)
	}

	if suggestion.Status != "pending" {
		return nil, fmt.Errorf("suggestion is not pending, current status: %s", suggestion.Status)
	}

	now := time.Now()
	endDate := now.AddDate(DefaultRecurrenceYears, 0, 0)

	// Build RRULE for weekly recurrence on the specified day of week
	dayMap := []string{"SU", "MO", "TU", "WE", "TH", "FR", "SA"}
	endDateUTC := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 23, 59, 59, 0, time.UTC)

	// Build RRULE for weekly recurrence on the specified day of week
	dayMap := []string{"SU", "MO", "TU", "WE", "TH", "FR", "SA"}
	byday := dayMap[suggestion.DayOfWeek]
	rrule := fmt.Sprintf("FREQ=WEEKLY;BYDAY=%s;UNTIL=%s", byday, endDateUTC.Format("20060102T150405Z"))

	// Calculate start and end times for the first occurrence
	daysUntilTarget := (suggestion.DayOfWeek - int(now.Weekday()) + 7) % 7
	if daysUntilTarget == 0 && now.Hour()*60+now.Minute() > suggestion.TimeOfDay {
		daysUntilTarget = 7
	}

	firstOccurrence := now.AddDate(0, 0, daysUntilTarget)
	eventStart := time.Date(
		firstOccurrence.Year(), firstOccurrence.Month(), firstOccurrence.Day(),
		suggestion.TimeOfDay/60, suggestion.TimeOfDay%60, 0, 0,
		firstOccurrence.Location(),
	)
	eventEnd := eventStart.Add(time.Duration(suggestion.DurationMinutes) * time.Minute)

	// Create recurring event as an Event record
	event := &models.Event{
		ID:             uuid.New().String(),
		UserID:         suggestion.UserID,
		Title:          suggestion.Title,
		Description:    suggestion.Description,
		Location:       suggestion.Location,
		StartTime:      eventStart,
		EndTime:        eventEnd,
		IsRecurring:    true,
		RecurrenceRule: rrule,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.repo.CreateRecurringEvent(ctx, event); err != nil {
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
		"event_id", event.ID,
		"end_date", endDate)

	return event, nil
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

// ===== HTTP Handlers =====

// handleGetSuggestions returns pending habit suggestions for the authenticated user
func (s *Service) handleGetSuggestions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"}); err != nil {
			s.logger.Error("Failed to encode JSON response", "error", err)
		}
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	suggestions, err := s.GetPendingSuggestions(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get habit suggestions", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "failed to get suggestions"}); err != nil {
			s.logger.Error("Failed to encode JSON response", "error", err)
		}
		return
	}

	// Convert to response format with readable time
	response := make([]map[string]interface{}, len(suggestions))
	for i, suggestion := range suggestions {
		response[i] = map[string]interface{}{
			"id":               suggestion.ID,
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

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode JSON response", "error", err)
	}
}

// handleAcceptSuggestion accepts a habit suggestion
func (s *Service) handleAcceptSuggestion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"}); err != nil {
			s.logger.Error("Failed to encode JSON response", "error", err)
		}
		return
	}

	suggestionID := mux.Vars(r)["id"]

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Verify the suggestion belongs to the authenticated user before accepting
	suggestion, err := s.repo.GetHabitSuggestionByID(ctx, suggestionID)
	if err != nil {
		s.logger.Error("Failed to get habit suggestion", "error", err)
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "suggestion not found"}); err != nil {
			s.logger.Error("Failed to encode JSON response", "error", err)
		}
		return
	}
	if suggestion.UserID != userID {
		w.WriteHeader(http.StatusForbidden)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"}); err != nil {
			s.logger.Error("Failed to encode JSON response", "error", err)
		}
		return
	}

	event, err := s.AcceptSuggestion(ctx, suggestionID)
	if err != nil {
		s.logger.Error("Failed to accept habit suggestion", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); err != nil {
			s.logger.Error("Failed to encode JSON response", "error", err)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success":         true,
		"message":         fmt.Sprintf("Habit accepted! '%s' will be scheduled weekly until %s", event.Title, event.EndTime.Format("January 2, 2006")),
		"recurring_event": event,
	}); err != nil {
		s.logger.Error("Failed to encode JSON response", "error", err)
	}
}

// handleRejectSuggestion rejects a habit suggestion
func (s *Service) handleRejectSuggestion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"}); err != nil {
			s.logger.Error("Failed to encode JSON response", "error", err)
		}
		return
	}

	suggestionID := mux.Vars(r)["id"]

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Verify the suggestion belongs to the authenticated user before rejecting
	suggestion, err := s.repo.GetHabitSuggestionByID(ctx, suggestionID)
	if err != nil {
		s.logger.Error("Failed to get habit suggestion", "error", err)
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "suggestion not found"}); err != nil {
			s.logger.Error("Failed to encode JSON response", "error", err)
		}
		return
	}
	if suggestion.UserID != userID {
		w.WriteHeader(http.StatusForbidden)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"}); err != nil {
			s.logger.Error("Failed to encode JSON response", "error", err)
		}
		return
	}

	if err := s.RejectSuggestion(ctx, suggestionID); err != nil {
		s.logger.Error("Failed to reject habit suggestion", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); err != nil {
			s.logger.Error("Failed to encode JSON response", "error", err)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Habit suggestion rejected",
	}); err != nil {
		s.logger.Error("Failed to encode JSON response", "error", err)
	}
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
