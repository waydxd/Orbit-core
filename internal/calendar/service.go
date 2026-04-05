package calendar

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
	"github.com/waydxd/Orbit-core/pkg/middleware"
	pb "github.com/waydxd/Orbit-core/proto/calendar"
)

// Service represents the Calendar & Task Service
type Service struct {
	pb.UnimplementedCalendarDataServiceServer
	pb.UnimplementedCalendarServiceServer
	config       *config.Config
	logger       *logger.Logger
	eventRepo    EventRepository
	taskRepo     TaskRepository
	habitTracker HabitTracker
}

// HabitTracker interface for tracking event patterns
type HabitTracker interface {
	TrackEventCreation(ctx context.Context, event *models.Event) error
}

// NewService creates a new Calendar Service
func NewService(cfg *config.Config, log *logger.Logger, eventRepo EventRepository, taskRepo TaskRepository, habitTracker HabitTracker) *Service {
	return &Service{
		config:       cfg,
		logger:       log,
		eventRepo:    eventRepo,
		taskRepo:     taskRepo,
		habitTracker: habitTracker,
	}
}

// RegisterRoutes registers calendar and task routes
func (s *Service) RegisterRoutes(router *mux.Router) {
	calendarRouter := router.PathPrefix("/calendar").Subrouter()

	// Event routes
	calendarRouter.HandleFunc("/events", s.listEvents).Methods("GET")
	calendarRouter.HandleFunc("/events", s.createEvent).Methods("POST")
	calendarRouter.HandleFunc("/events/{id}", s.getEvent).Methods("GET")
	calendarRouter.HandleFunc("/events/{id}", s.updateEvent).Methods("PUT")
	calendarRouter.HandleFunc("/events/{id}", s.deleteEvent).Methods("DELETE")

	// Recurring event routes
	calendarRouter.HandleFunc("/recurring", s.listRecurringEvents).Methods("GET")
	calendarRouter.HandleFunc("/recurring/{id}/deactivate", s.deactivateRecurringEvent).Methods("POST")

	// Task routes
	calendarRouter.HandleFunc("/tasks", s.listTasks).Methods("GET")
	calendarRouter.HandleFunc("/tasks", s.createTask).Methods("POST")
	calendarRouter.HandleFunc("/tasks/{id}", s.getTask).Methods("GET")
	calendarRouter.HandleFunc("/tasks/{id}", s.updateTask).Methods("PUT")
	calendarRouter.HandleFunc("/tasks/{id}", s.deleteTask).Methods("DELETE")
}

// ===== Event HTTP Handlers =====

// listEvents retrieves events for a user
func (s *Service) listEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"}); err != nil {
			s.logger.Error("failed to write listEvents error response", "error", err)
			return
		}
		return
	}

	startTimeStr := r.URL.Query().Get("start_time")
	endTimeStr := r.URL.Query().Get("end_time")

	var startTime, endTime time.Time
	var err error

	if startTimeStr != "" {
		startTime, err = time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid start_time format"})
			return
		}
	}

	if endTimeStr != "" {
		endTime, err = time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid end_time format"})
			return
		}
	}

	// Default window: if BOTH are missing or empty, use 3-month window (previous, current, next month).
	// If only one is provided, use sensible defaults (epoch or far future).
	if startTime.IsZero() && endTime.IsZero() {
		now := time.Now().UTC()
		// Start of previous month. time.Date normalizes out-of-range months (e.g., month 0 -> Dec of previous year),
		// so now.Month()-1 correctly handles the January -> previous December rollover.
		startTime = time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
		// End of next month (last instant before the following month starts)
		endTime = time.Date(now.Year(), now.Month()+2, 1, 0, 0, 0, -1, time.UTC)
	} else {
		if startTime.IsZero() {
			startTime = time.Unix(0, 0)
		}
		if endTime.IsZero() {
			endTime = time.Now().AddDate(10, 0, 0)
		}
	}

	if endTime.Before(startTime) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "end_time must be after start_time"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Log query parameters for debugging
	s.logger.Info("Listing events", "user_id", userID, "start_time", startTime, "end_time", endTime)

	events, err := s.eventRepo.ListEvents(ctx, userID, startTime, endTime)
	if err != nil {
		s.logger.Error("failed to list events", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "failed to list events"}); err != nil {
			s.logger.Error("failed to write listEvents error response", "error", err)
			return
		}
		return
	}

	if err := json.NewEncoder(w).Encode(events); err != nil {
		s.logger.Error("failed to write listEvents response", "error", err)
		return
	}
}

// createEvent creates a new event
func (s *Service) createEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	var req struct {
		Title               string   `json:"title"`
		Description         string   `json:"description"`
		StartTime           string   `json:"start_time"`
		EndTime             string   `json:"end_time"`
		Location            string   `json:"location"`
		Hashtags            []string `json:"hashtags"`
		IsRecurring         bool     `json:"is_recurring"`
		RecurrenceRule      string   `json:"recurrence_rule"`
		RecurrenceException string   `json:"recurrence_exception"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"}); err != nil {
			s.logger.Error("failed to write createEvent error response", "error", err)
			return
		}
		return
	}

	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "invalid start_time format"}); err != nil {
			s.logger.Error("failed to write createEvent error response", "error", err)
			return
		}
		return
	}

	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "invalid end_time format"}); err != nil {
			s.logger.Error("failed to write createEvent error response", "error", err)
			return
		}
		return
	}

	event := &models.Event{
		ID:                  uuid.New().String(),
		UserID:              userID,
		Title:               req.Title,
		Description:         req.Description,
		StartTime:           startTime,
		EndTime:             endTime,
		Location:            req.Location,
		Hashtags:            req.Hashtags,
		IsRecurring:         req.IsRecurring,
		RecurrenceRule:      req.RecurrenceRule,
		RecurrenceException: req.RecurrenceException,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := s.eventRepo.CreateEvent(ctx, event); err != nil {
		s.logger.Error("failed to create event", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "failed to create event"}); err != nil {
			s.logger.Error("failed to write createEvent error response", "error", err)
			return
		}
		return
	}

	// Track event for habit detection (async, don't block response)
	// Skip habit tracking for events already marked as recurring or having a recurrence rule to avoid redundant suggestions
	if s.habitTracker != nil && !event.IsRecurring && event.RecurrenceRule == "" {
		trackParentCtx := context.WithoutCancel(r.Context())
		go func() {
			trackCtx, trackCancel := context.WithTimeout(trackParentCtx, 5*time.Second)
			defer trackCancel()
			if err := s.habitTracker.TrackEventCreation(trackCtx, event); err != nil {
				s.logger.Error("failed to track event for habit detection", "err", err)
			}
		}()
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(event); err != nil {
		s.logger.Error("failed to write createEvent success response", "error", err)
		return
	}
}

// getEvent retrieves an event by ID
func (s *Service) getEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	id := mux.Vars(r)["id"]
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	event, err := s.eventRepo.GetEventByID(ctx, id)
	if err != nil || event.UserID != userID {
		s.logger.Error("failed to get event or unauthorized", "err", err)
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "event not found"}); err != nil {
			s.logger.Error("failed to write getEvent error response", "error", err)
			return
		}
		return
	}

	if err := json.NewEncoder(w).Encode(event); err != nil {
		s.logger.Error("failed to write getEvent response", "error", err)
		return
	}
}

// updateEvent updates an existing event
func (s *Service) updateEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	var req updateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	// Verify ownership
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	existing, err := s.eventRepo.GetEventByID(ctx, id)
	if err != nil || existing.UserID != userID {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "event not found"})
		return
	}

	// Apply updates (helper handles parsing & optional fields)
	applyUpdateToEvent(existing, &req)

	if err := s.eventRepo.UpdateEvent(ctx, existing); err != nil {
		s.logger.Error("failed to update event", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "failed to update event"}); err != nil {
			s.logger.Error("failed to write updateEvent error response", "error", err)
			return
		}
		return
	}

	if s.habitTracker != nil && !existing.IsRecurring && existing.RecurrenceRule == "" {
		trackParentCtx := context.WithoutCancel(ctx)
		go func() {
			trackCtx, trackCancel := context.WithTimeout(trackParentCtx, 5*time.Second)
			defer trackCancel()
			if err := s.habitTracker.TrackEventCreation(trackCtx, existing); err != nil {
				s.logger.Error("failed to track event for habit detection (API update)", "err", err)
			}
		}()
	}

	if err := json.NewEncoder(w).Encode(existing); err != nil {
		s.logger.Error("failed to write updateEvent response", "error", err)
		return
	}
}

// deleteEvent deletes an event
func (s *Service) deleteEvent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Verify ownership before delete
	existing, err := s.eventRepo.GetEventByID(ctx, id)
	if err != nil || existing.UserID != userID {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "event not found"})
		return
	}

	if err := s.eventRepo.DeleteEvent(ctx, id); err != nil {
		s.logger.Error("failed to delete event", "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to delete event"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ===== Recurring Event HTTP Handlers =====

// listRecurringEvents returns active recurring events for a user
func (s *Service) listRecurringEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	recurring, err := s.eventRepo.GetActiveRecurringEvents(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get recurring events", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to get recurring events"})
		return
	}

	// Convert to response format
	response := make([]map[string]interface{}, len(recurring))
	for i, evt := range recurring {
		durationMinutes := int(evt.EndTime.Sub(evt.StartTime).Minutes())
		response[i] = map[string]interface{}{
			"id":               evt.ID,
			"user_id":          evt.UserID,
			"title":            evt.Title,
			"description":      evt.Description,
			"location":         evt.Location,
			"start_time":       evt.StartTime,
			"end_time":         evt.EndTime,
			"duration_minutes": durationMinutes,
			"is_recurring":     evt.IsRecurring,
			"recurrence_rule":  evt.RecurrenceRule,
			"created_at":       evt.CreatedAt,
		}
	}

	_ = json.NewEncoder(w).Encode(response)
}

// deactivateRecurringEvent deactivates a recurring event
func (s *Service) deactivateRecurringEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	recurringID := mux.Vars(r)["id"]

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := s.eventRepo.DeactivateRecurringEvent(ctx, recurringID); err != nil {
		s.logger.Error("Failed to deactivate recurring event", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Recurring event deactivated",
	})
}

// ===== Task HTTP Handlers =====

// listTasks retrieves tasks for a user
func (s *Service) listTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	completedStr := r.URL.Query().Get("completed")
	var completed *bool
	if completedStr != "" {
		val := completedStr == "true"
		completed = &val
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	tasks, err := s.taskRepo.ListTasks(ctx, userID, completed)
	if err != nil {
		s.logger.Error("failed to list tasks", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "failed to list tasks"}); err != nil {
			s.logger.Error("failed to write listTasks error response", "error", err)
			return
		}
		return
	}

	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		s.logger.Error("failed to write listTasks response", "error", err)
		return
	}
}

// createTask creates a new task
func (s *Service) createTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	var req struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		DueDate     string   `json:"due_date"`
		Priority    string   `json:"priority"`
		Hashtags    []string `json:"hashtags"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"}); err != nil {
			s.logger.Error("failed to write createTask error response", "error", err)
			return
		}
		return
	}

	dueDate := time.Time{}
	if req.DueDate != "" {
		if t, err := time.Parse(time.RFC3339, req.DueDate); err == nil {
			dueDate = t
		}
	}

	task := &models.Task{
		ID:          uuid.New().String(),
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
		DueDate:     dueDate,
		Priority:    req.Priority,
		Hashtags:    req.Hashtags,
		Completed:   false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := s.taskRepo.CreateTask(ctx, task); err != nil {
		s.logger.Error("failed to create task", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "failed to create task"}); err != nil {
			s.logger.Error("failed to write createTask error response", "error", err)
			return
		}
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(task); err != nil {
		s.logger.Error("failed to write createTask success response", "error", err)
		return
	}
}

// getTask retrieves a task by ID
func (s *Service) getTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	id := mux.Vars(r)["id"]
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	task, err := s.taskRepo.GetTaskByID(ctx, id)
	if err != nil || task.UserID != userID {
		s.logger.Error("failed to get task or unauthorized", "err", err)
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "task not found"}); err != nil {
			s.logger.Error("failed to write getTask error response", "error", err)
			return
		}
		return
	}

	if err := json.NewEncoder(w).Encode(task); err != nil {
		s.logger.Error("failed to write getTask response", "error", err)
		return
	}
}

// updateTask updates an existing task
func (s *Service) updateTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	var req struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		DueDate     string   `json:"due_date"`
		Completed   bool     `json:"completed"`
		Priority    string   `json:"priority"`
		Hashtags    []string `json:"hashtags"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	// Verify ownership
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	existing, err := s.taskRepo.GetTaskByID(ctx, id)
	if err != nil || existing.UserID != userID {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "task not found"})
		return
	}

	if req.Title != "" {
		existing.Title = req.Title
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	if req.Priority != "" {
		existing.Priority = req.Priority
	}
	if req.DueDate != "" {
		if t, err := time.Parse(time.RFC3339, req.DueDate); err == nil {
			existing.DueDate = t
		}
	}
	if req.Hashtags != nil {
		existing.Hashtags = req.Hashtags
	}
	existing.Completed = req.Completed

	if err := s.taskRepo.UpdateTask(ctx, existing); err != nil {
		s.logger.Error("failed to update task", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "failed to update task"}); err != nil {
			s.logger.Error("failed to write updateTask error response", "error", err)
			return
		}
		return
	}

	if err := json.NewEncoder(w).Encode(existing); err != nil {
		s.logger.Error("failed to write updateTask response", "error", err)
		return
	}
}

// deleteTask deletes a task
func (s *Service) deleteTask(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Verify ownership
	existing, err := s.taskRepo.GetTaskByID(ctx, id)
	if err != nil || existing.UserID != userID {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "task not found"})
		return
	}

	if err := s.taskRepo.DeleteTask(ctx, id); err != nil {
		s.logger.Error("failed to delete task", "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to delete task"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type updateEventRequest struct {
	Title               string   `json:"title,omitempty"`
	Description         string   `json:"description,omitempty"`
	StartTime           string   `json:"start_time,omitempty"`
	EndTime             string   `json:"end_time,omitempty"`
	Location            string   `json:"location,omitempty"`
	Hashtags            []string `json:"hashtags,omitempty"`
	IsRecurring         bool     `json:"is_recurring,omitempty"`
	RecurrenceRule      string   `json:"recurrence_rule,omitempty"`
	RecurrenceException string   `json:"recurrence_exception,omitempty"`
}
