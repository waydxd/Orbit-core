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
	// Skip tracking if the event is already recurring
	if event.RecurrenceRule != "" || event.IsRecurring {
		s.logger.Debug("Skipping habit track for recurring event", "event_id", event.ID)
		return nil
	}

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
		// If suggestion was already shown (accepted or rejected) previously,
		// and user is manually creating these events AGAIN, treat this as a new cycle.
		// Since we already marked past events as recurring, hitting this means new non-recurring events were just created.
		if existing.SuggestionShown {
			existing.OccurrenceCount = 1
			existing.OccurrenceTimestamps = []time.Time{event.StartTime}
			existing.SuggestionShown = false
			existing.HabitAccepted = false
		} else {
			// Update existing frequency record
			existing.OccurrenceCount++
			existing.OccurrenceTimestamps = append(existing.OccurrenceTimestamps, event.StartTime)
			// Keep only the last 10 timestamps
			if len(existing.OccurrenceTimestamps) > 10 {
				existing.OccurrenceTimestamps = existing.OccurrenceTimestamps[len(existing.OccurrenceTimestamps)-10:]
			}
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

	// Mark matched events from this pattern as recurring immediately (per requirement)
	if err := s.repo.MarkEventsAsRecurringByPattern(ctx, freq.UserID, freq.Title, freq.DayOfWeek, freq.TimeOfDay, freq.DurationMinutes); err != nil {
		s.logger.Warn("Failed to mark existing events as recurring", "error", err, "pattern_title", freq.Title)
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

// calculateTotalWeeks helper
func calculateTotalWeeks(customYears *int, customWeeks *int) int {
	totalWeeks := 0
	if customYears != nil {
		totalWeeks += *customYears * 52
	} else if customWeeks == nil {
		totalWeeks = DefaultRecurrenceYears * 52
	}
	if customWeeks != nil {
		totalWeeks += *customWeeks
	}
	if totalWeeks <= 0 {
		totalWeeks = 1
	}
	return totalWeeks
}

// calculateEventStart calculates the start time for the first occurrence
func (s *Service) calculateEventStart(ctx context.Context, suggestion *models.HabitSuggestion, now time.Time) time.Time {
	freq, _ := s.repo.GetEventFrequencyByPattern(ctx, suggestion.UserID, suggestion.Title, suggestion.DurationMinutes, suggestion.TimeOfDay, suggestion.DayOfWeek)
	if freq != nil && len(freq.OccurrenceTimestamps) > 0 {
		lastOccurrence := freq.OccurrenceTimestamps[len(freq.OccurrenceTimestamps)-1]
		return time.Date(
			lastOccurrence.Year(), lastOccurrence.Month(), lastOccurrence.Day(),
			suggestion.TimeOfDay/60, suggestion.TimeOfDay%60, 0, 0,
			lastOccurrence.Location(),
		).AddDate(0, 0, 7)
	}

	daysUntilTarget := (suggestion.DayOfWeek - int(now.Weekday()) + 7) % 7
	if daysUntilTarget == 0 && now.Hour()*60+now.Minute() > suggestion.TimeOfDay {
		daysUntilTarget = 7
	}
	firstOccurrence := now.AddDate(0, 0, daysUntilTarget)
	return time.Date(
		firstOccurrence.Year(), firstOccurrence.Month(), firstOccurrence.Day(),
		suggestion.TimeOfDay/60, suggestion.TimeOfDay%60, 0, 0,
		firstOccurrence.Location(),
	)
}

// AcceptSuggestion accepts a habit suggestion and creates a recurring event
func (s *Service) AcceptSuggestion(ctx context.Context, suggestionID string, customYears *int, customWeeks *int) (*models.Event, error) {
	suggestion, err := s.repo.GetHabitSuggestionByID(ctx, suggestionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get suggestion: %w", err)
	}
	if suggestion == nil {
		return nil, fmt.Errorf("suggestion not found")
	}
	if suggestion.Status != "pending" {
		return nil, fmt.Errorf("suggestion is not pending, current status: %s", suggestion.Status)
	}

	totalWeeks := calculateTotalWeeks(customYears, customWeeks)

	now := time.Now()
	eventStart := s.calculateEventStart(ctx, suggestion, now)

	var firstEvent *models.Event
	var lastEndDate time.Time

	for i := 0; i < totalWeeks; i++ {
		currentStart := eventStart.AddDate(0, 0, i*7)
		currentEnd := currentStart.Add(time.Duration(suggestion.DurationMinutes) * time.Minute)

		event := &models.Event{
			ID:             uuid.New().String(),
			UserID:         suggestion.UserID,
			Title:          suggestion.Title,
			Description:    suggestion.Description,
			Location:       suggestion.Location,
			StartTime:      currentStart,
			EndTime:        currentEnd,
			IsRecurring:    true,
			RecurrenceRule: "",
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		if err := s.repo.CreateRecurringEvent(ctx, event); err != nil {
			return nil, fmt.Errorf("failed to create recurring event occurrence %d: %w", i, err)
		}

		if i == 0 {
			firstEvent = event
		}
		lastEndDate = currentEnd
	}

	// Update suggestion status
	if err := s.repo.UpdateHabitSuggestionStatus(ctx, suggestionID, "accepted", &lastEndDate); err != nil {
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
	s.logger.Info("Accepted habit suggestion and created recurring events",
		"suggestion_id", suggestionID,
		"total_weeks", totalWeeks,
		"end_date", lastEndDate)
	return firstEvent, nil
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

	// Fetch user timezone
	userTz, err := s.repo.GetUserTimezone(ctx, userID)
	if err != nil {
		s.logger.Warn("Failed to get user timezone, using HKT", "error", err)
		userTz = "Asia/Hong_Kong"
	}
	location, err := time.LoadLocation(userTz)
	if err != nil {
		s.logger.Warn("Invalid timezone, using HKT", "timezone", userTz)
		location, _ = time.LoadLocation("Asia/Hong_Kong")
	}

	// Convert to response format with readable time
	response := make([]map[string]interface{}, len(suggestions))
	for i, suggestion := range suggestions {
		// Calculate precise target date based on last occurrence
		var suggestedStart, suggestedEnd time.Time
		freq, _ := s.repo.GetEventFrequencyByPattern(ctx, suggestion.UserID, suggestion.Title, suggestion.DurationMinutes, suggestion.TimeOfDay, suggestion.DayOfWeek)
		if freq != nil && len(freq.OccurrenceTimestamps) > 0 {
			lastOccurrence := freq.OccurrenceTimestamps[len(freq.OccurrenceTimestamps)-1]
			suggestedStart = time.Date(lastOccurrence.Year(), lastOccurrence.Month(), lastOccurrence.Day(), suggestion.TimeOfDay/60, suggestion.TimeOfDay%60, 0, 0, lastOccurrence.Location()).AddDate(0, 0, 7)
		} else {
			// Fallback calculate next day of week from suggestion creation (or now)
			now := time.Now()
			daysUntilTarget := (suggestion.DayOfWeek - int(now.Weekday()) + 7) % 7
			if daysUntilTarget == 0 && now.Hour()*60+now.Minute() > suggestion.TimeOfDay {
				daysUntilTarget = 7
			}
			fallbackTarget := now.AddDate(0, 0, daysUntilTarget)
			suggestedStart = time.Date(fallbackTarget.Year(), fallbackTarget.Month(), fallbackTarget.Day(), suggestion.TimeOfDay/60, suggestion.TimeOfDay%60, 0, 0, fallbackTarget.Location())
		}
		suggestedEnd = suggestedStart.Add(time.Duration(suggestion.DurationMinutes) * time.Minute)

		suggestedStartLoc := suggestedStart.In(location)
		localTimeOfDay := suggestedStartLoc.Hour()*60 + suggestedStartLoc.Minute()
		localDayOfWeek := int(suggestedStartLoc.Weekday())

		response[i] = map[string]interface{}{
			"id":                   suggestion.ID,
			"title":                suggestion.Title,
			"description":          suggestion.Description,
			"location":             suggestion.Location,
			"duration_minutes":     suggestion.DurationMinutes,
			"time_of_day":          formatTimeOfDay(localTimeOfDay),
			"day_of_week":          formatDayOfWeek(localDayOfWeek),
			"status":               suggestion.Status,
			"created_at":           suggestion.CreatedAt,
			"expires_at":           suggestion.ExpiresAt,
			"suggested_start_time": suggestedStart.Format(time.RFC3339),
			"suggested_end_time":   suggestedEnd.Format(time.RFC3339),
			"message":              fmt.Sprintf("You've scheduled '%s' multiple times on %s at %s. Would you like to make this a recurring event in the future?", suggestion.Title, formatDayOfWeek(localDayOfWeek), formatTimeOfDay(localTimeOfDay)),
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
	var req struct {
		Years *int `json:"years"`
		Weeks *int `json:"weeks"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	event, err := s.AcceptSuggestion(ctx, suggestionID, req.Years, req.Weeks)
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
		"message":         fmt.Sprintf("Habit accepted! '%s' will be scheduled for the selected duration", event.Title),
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
