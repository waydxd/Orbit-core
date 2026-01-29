package calendar

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
	GetRecurringEventsForTimeRange(ctx context.Context, userID string, startTime, endTime time.Time) ([]*models.Event, error)
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
		Title               string `json:"title"`
		Description         string `json:"description"`
		StartTime           string `json:"start_time"`
		EndTime             string `json:"end_time"`
		Location            string `json:"location"`
		IsRecurring         bool   `json:"is_recurring"`
		RecurrenceRule      string `json:"recurrence_rule"`
		RecurrenceException string `json:"recurrence_exception"`
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
	if s.habitTracker != nil {
		go func() {
			trackCtx, trackCancel := context.WithTimeout(context.Background(), 5*time.Second)
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
		Title       string `json:"title"`
		Description string `json:"description"`
		DueDate     string `json:"due_date"`
		Priority    string `json:"priority"`
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
		Title       string `json:"title"`
		Description string `json:"description"`
		DueDate     string `json:"due_date"`
		Completed   bool   `json:"completed"`
		Priority    string `json:"priority"`
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

// ===== gRPC Server Implementation =====

// GetCalendarData implements CalendarDataService.GetCalendarData
func (s *Service) GetCalendarData(ctx context.Context, req *pb.GetCalendarDataRequest) (*pb.GetCalendarDataResponse, error) {
	s.logger.Info("GetCalendarData called by Agent", "user_id", req.UserId)

	startTime := time.Unix(req.StartTime, 0)
	endTime := time.Unix(req.EndTime, 0)

	events, err := s.eventRepo.ListEvents(ctx, req.UserId, startTime, endTime)
	if err != nil {
		s.logger.Error("failed to get calendar data", "err", err)
		return &pb.GetCalendarDataResponse{
			Success: false,
			Message: fmt.Sprintf("failed to get calendar data: %v", err),
		}, nil
	}

	pbEvents := make([]*pb.Event, len(events))
	for i, event := range events {
		pbEvents[i] = &pb.Event{
			Id:          event.ID,
			Title:       event.Title,
			Description: event.Description,
			StartTime:   event.StartTime.Unix(),
			EndTime:     event.EndTime.Unix(),
			Location:    event.Location,
		}
	}

	return &pb.GetCalendarDataResponse{
		Events:  pbEvents,
		Success: true,
		Message: "Calendar data retrieved successfully",
	}, nil
}

// GetUserAvailability implements CalendarDataService.GetUserAvailability
func (s *Service) GetUserAvailability(ctx context.Context, req *pb.GetUserAvailabilityRequest) (*pb.GetUserAvailabilityResponse, error) {
	s.logger.Info("GetUserAvailability called by Agent", "user_id", req.UserId)

	startTime := time.Unix(req.StartTime, 0)
	endTime := time.Unix(req.EndTime, 0)

	events, err := s.eventRepo.ListEvents(ctx, req.UserId, startTime, endTime)
	if err != nil {
		s.logger.Error("failed to check availability", "err", err)
		return &pb.GetUserAvailabilityResponse{
			Success: false,
			Message: fmt.Sprintf("failed to check availability: %v", err),
		}, nil
	}

	available := len(events) == 0
	reason := "No conflicts found"
	if !available {
		reason = fmt.Sprintf("%d conflicting events found", len(events))
	}

	pbEvents := make([]*pb.Event, len(events))
	for i, event := range events {
		pbEvents[i] = &pb.Event{
			Id:          event.ID,
			Title:       event.Title,
			Description: event.Description,
			StartTime:   event.StartTime.Unix(),
			EndTime:     event.EndTime.Unix(),
			Location:    event.Location,
		}
	}

	status := &pb.AvailabilityStatus{
		Available:         available,
		Reason:            reason,
		ConflictingEvents: pbEvents,
	}

	return &pb.GetUserAvailabilityResponse{
		Status:  status,
		Success: true,
		Message: "Availability check completed",
	}, nil
}

// QueryEvents implements CalendarDataService.QueryEvents
func (s *Service) QueryEvents(ctx context.Context, req *pb.QueryEventsRequest) (*pb.QueryEventsResponse, error) {
	s.logger.Info("QueryEvents called by Agent", "user_id", req.UserId)

	startTime := time.Unix(req.StartTime, 0)
	endTime := time.Unix(req.EndTime, 0)

	events, err := s.eventRepo.ListEvents(ctx, req.UserId, startTime, endTime)
	if err != nil {
		s.logger.Error("failed to query events", "err", err)
		return &pb.QueryEventsResponse{
			Success: false,
			Message: fmt.Sprintf("failed to query events: %v", err),
		}, nil
	}

	pbEvents := make([]*pb.Event, len(events))
	for i, event := range events {
		pbEvents[i] = &pb.Event{
			Id:          event.ID,
			Title:       event.Title,
			Description: event.Description,
			StartTime:   event.StartTime.Unix(),
			EndTime:     event.EndTime.Unix(),
			Location:    event.Location,
		}
	}

	// Safely convert length to int32 avoiding overflow
	var totalCount int32
	if len(pbEvents) > int(^uint32(0)>>1) { // larger than max int32
		totalCount = -1 // indicate overflow
	} else {
		// #nosec G115 -- len(pbEvents) is guaranteed to be within int32 range
		totalCount = int32(len(pbEvents))
	}

	return &pb.QueryEventsResponse{
		Events:     pbEvents,
		TotalCount: totalCount,
		Success:    true,
		Message:    "Query executed successfully",
	}, nil
}

// Adapter methods to satisfy agent.CalendarServiceInterface and gateway.CalendarServiceInterface

// ListEventsAdapter returns events across users (userID omitted) or can be extended to filter by status.
func (s *Service) ListEventsAdapter(ctx context.Context, startTime, endTime int64, status string) ([]interface{}, error) {
	_ = status // mark as used until filtering is implemented
	st := time.Unix(startTime, 0)
	en := time.Unix(endTime, 0)

	events, err := s.eventRepo.ListEvents(ctx, "", st, en)
	if err != nil {
		s.logger.Error("failed to list events (adapter)", "err", err)
		return nil, err
	}

	out := make([]interface{}, len(events))
	for i, e := range events {
		out[i] = e
	}
	return out, nil
}

// CreateEventAdapter accepts flexible payloads (map[string]interface{}, *models.Event, pb.Event) and creates an event
func (s *Service) CreateEventAdapter(ctx context.Context, event interface{}) (interface{}, error) {
	// Delegate parsing to a helper to keep cyclomatic complexity low
	ev, err := parseEventPayload(event)
	if err != nil {
		s.logger.Error("failed to parse event payload (adapter)", "err", err)
		return nil, err
	}

	if ev.ID == "" {
		ev.ID = uuid.New().String()
	}
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now()
	}
	ev.UpdatedAt = time.Now()

	if err := s.eventRepo.CreateEvent(ctx, &ev); err != nil {
		s.logger.Error("failed to create event (adapter)", "err", err)
		return nil, err
	}

	// Track event for habit detection (async, don't block response)
	if s.habitTracker != nil {
		go func() {
			trackCtx, trackCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer trackCancel()
			if err := s.habitTracker.TrackEventCreation(trackCtx, &ev); err != nil {
				s.logger.Error("failed to track event for habit detection (adapter)", "err", err)
			}
		}()
	}

	return &ev, nil
}

// parseEventPayload converts supported input types into a models.Event value.
// Supported input types: map[string]interface{}, *models.Event, models.Event, *pb.Event
func parseEventPayload(event interface{}) (models.Event, error) {
	var ev models.Event
	switch v := event.(type) {
	case map[string]interface{}:
		if id, ok := v["user_id"].(string); ok {
			ev.UserID = id
		}
		if title, ok := v["title"].(string); ok {
			ev.Title = title
		}
		if desc, ok := v["description"].(string); ok {
			ev.Description = desc
		}
		if loc, ok := v["location"].(string); ok {
			ev.Location = loc
		}
		// Parse start_time and end_time using helper
		if st, ok := v["start_time"]; ok {
			if t, err := parseTimeFromInterface(st); err == nil {
				ev.StartTime = t
			}
		}
		if et, ok := v["end_time"]; ok {
			if t, err := parseTimeFromInterface(et); err == nil {
				ev.EndTime = t
			}
		}
	case *models.Event:
		ev = *v
	case models.Event:
		ev = v
	case *pb.Event:
		ev.ID = v.Id
		ev.Title = v.Title
		ev.Description = v.Description
		ev.StartTime = time.Unix(v.StartTime, 0)
		ev.EndTime = time.Unix(v.EndTime, 0)
		ev.Location = v.Location
	default:
		return models.Event{}, fmt.Errorf("unsupported event type")
	}
	return ev, nil
}

// parseTimeFromInterface parses time from various interface{} types (int64, float64, string)
func parseTimeFromInterface(v interface{}) (time.Time, error) {
	switch tv := v.(type) {
	case int64:
		return time.Unix(tv, 0), nil
	case float64:
		return time.Unix(int64(tv), 0), nil
	case string:
		return time.Parse(time.RFC3339, tv)
	default:
		return time.Time{}, fmt.Errorf("unsupported time type")
	}
}

// UpdateEventAdapter updates an event by id using flexible payloads
func (s *Service) UpdateEventAdapter(ctx context.Context, id string, event interface{}) (interface{}, error) {
	existing, err := s.eventRepo.GetEventByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get event for update", "id", id, "err", err)
		return nil, err
	}

	switch v := event.(type) {
	case map[string]interface{}:
		if title, ok := v["title"].(string); ok {
			existing.Title = title
		}
		if desc, ok := v["description"].(string); ok {
			existing.Description = desc
		}
		if loc, ok := v["location"].(string); ok {
			existing.Location = loc
		}
		if sts, ok := v["start_time"].(string); ok {
			if t, err := time.Parse(time.RFC3339, sts); err == nil {
				existing.StartTime = t
			}
		}
		if ets, ok := v["end_time"].(string); ok {
			if t, err := time.Parse(time.RFC3339, ets); err == nil {
				existing.EndTime = t
			}
		}
	case *models.Event:
		// Replace the existing pointer with provided pointer
		existing = v
	case models.Event:
		// Copy fields from value into existing pointer
		existing.Title = v.Title
		existing.Description = v.Description
		existing.StartTime = v.StartTime
		existing.EndTime = v.EndTime
		existing.Location = v.Location
	case *pb.Event:
		existing.Title = v.Title
		existing.Description = v.Description
		existing.StartTime = time.Unix(v.StartTime, 0)
		existing.EndTime = time.Unix(v.EndTime, 0)
		existing.Location = v.Location
	default:
		return nil, fmt.Errorf("unsupported event type for update")
	}

	existing.UpdatedAt = time.Now()
	if err := s.eventRepo.UpdateEvent(ctx, existing); err != nil {
		s.logger.Error("failed to update event (adapter)", "err", err)
		return nil, err
	}
	return existing, nil
}

// DeleteEventAdapter deletes an event by id
func (s *Service) DeleteEventAdapter(ctx context.Context, id string) error {
	if err := s.eventRepo.DeleteEvent(ctx, id); err != nil {
		s.logger.Error("failed to delete event (adapter)", "id", id, "err", err)
		return err
	}
	return nil
}

// CreateEvent implements CalendarService.CreateEvent
func (s *Service) CreateEvent(ctx context.Context, req *pb.CreateEventRequest) (*pb.CreateEventResponse, error) {
	s.logger.Info("CreateEvent called via gRPC", "user_id", req.UserId)

	event := &models.Event{
		ID:          uuid.New().String(),
		UserID:      req.UserId,
		Title:       req.Title,
		Description: req.Description,
		StartTime:   time.Unix(req.StartTime, 0),
		EndTime:     time.Unix(req.EndTime, 0),
		Location:    req.Location,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.eventRepo.CreateEvent(ctx, event); err != nil {
		s.logger.Error("failed to create event via gRPC", "err", err)
		return &pb.CreateEventResponse{
			Success: false,
			Message: fmt.Sprintf("failed to create event: %v", err),
		}, nil
	}

	// Track event for habit detection (async, don't block response)
	if s.habitTracker != nil {
		go func() {
			trackCtx, trackCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer trackCancel()
			if err := s.habitTracker.TrackEventCreation(trackCtx, event); err != nil {
				s.logger.Error("failed to track event for habit detection (gRPC)", "err", err)
			}
		}()
	}

	pbEvent := &pb.Event{
		Id:          event.ID,
		UserId:      event.UserID,
		Title:       event.Title,
		Description: event.Description,
		StartTime:   event.StartTime.Unix(),
		EndTime:     event.EndTime.Unix(),
		Location:    event.Location,
	}

	return &pb.CreateEventResponse{
		Event:   pbEvent,
		Success: true,
		Message: "Event created successfully",
	}, nil
}

// GetEvents implements CalendarService.GetEvents
func (s *Service) GetEvents(ctx context.Context, req *pb.GetEventsRequest) (*pb.GetEventsResponse, error) {
	s.logger.Info("GetEvents called via gRPC", "user_id", req.UserId)

	startTime := time.Unix(req.StartTime, 0)
	endTime := time.Unix(req.EndTime, 0)

	events, err := s.eventRepo.ListEvents(ctx, req.UserId, startTime, endTime)
	if err != nil {
		s.logger.Error("failed to get events via gRPC", "err", err)
		return &pb.GetEventsResponse{
			Success: false,
			Message: fmt.Sprintf("failed to get events: %v", err),
		}, nil
	}

	pbEvents := make([]*pb.Event, len(events))
	for i, event := range events {
		pbEvents[i] = &pb.Event{
			Id:          event.ID,
			UserId:      event.UserID,
			Title:       event.Title,
			Description: event.Description,
			StartTime:   event.StartTime.Unix(),
			EndTime:     event.EndTime.Unix(),
			Location:    event.Location,
		}
	}

	return &pb.GetEventsResponse{
		Events:  pbEvents,
		Success: true,
		Message: "Events retrieved successfully",
	}, nil
}

// UpdateEvent implements CalendarService.UpdateEvent
func (s *Service) UpdateEvent(ctx context.Context, req *pb.UpdateEventRequest) (*pb.UpdateEventResponse, error) {
	s.logger.Info("UpdateEvent called via gRPC", "user_id", req.UserId, "event_id", req.Id)

	event, err := s.eventRepo.GetEventByID(ctx, req.Id)
	if err != nil {
		return &pb.UpdateEventResponse{
			Success: false,
			Message: fmt.Sprintf("event not found: %v", err),
		}, nil
	}

	if req.Title != "" {
		event.Title = req.Title
	}
	if req.Description != "" {
		event.Description = req.Description
	}
	if req.Location != "" {
		event.Location = req.Location
	}
	if req.StartTime != 0 {
		event.StartTime = time.Unix(req.StartTime, 0)
	}
	if req.EndTime != 0 {
		event.EndTime = time.Unix(req.EndTime, 0)
	}
	event.UpdatedAt = time.Now()

	if err := s.eventRepo.UpdateEvent(ctx, event); err != nil {
		s.logger.Error("failed to update event via gRPC", "err", err)
		return &pb.UpdateEventResponse{
			Success: false,
			Message: fmt.Sprintf("failed to update event: %v", err),
		}, nil
	}

	pbEvent := &pb.Event{
		Id:          event.ID,
		UserId:      event.UserID,
		Title:       event.Title,
		Description: event.Description,
		StartTime:   event.StartTime.Unix(),
		EndTime:     event.EndTime.Unix(),
		Location:    event.Location,
	}

	return &pb.UpdateEventResponse{
		Event:   pbEvent,
		Success: true,
		Message: "Event updated successfully",
	}, nil
}

// DeleteEvent implements CalendarService.DeleteEvent
func (s *Service) DeleteEvent(ctx context.Context, req *pb.DeleteEventRequest) (*pb.DeleteEventResponse, error) {
	s.logger.Info("DeleteEvent called via gRPC", "user_id", req.UserId, "event_id", req.Id)

	if err := s.eventRepo.DeleteEvent(ctx, req.Id); err != nil {
		s.logger.Error("failed to delete event via gRPC", "err", err)
		return &pb.DeleteEventResponse{
			Success: false,
			Message: fmt.Sprintf("failed to delete event: %v", err),
		}, nil
	}

	return &pb.DeleteEventResponse{
		Success: true,
		Message: "Event deleted successfully",
	}, nil
}

// GetAvailableSlots implements CalendarService.GetAvailableSlots
func (s *Service) GetAvailableSlots(ctx context.Context, req *pb.GetAvailableSlotsRequest) (*pb.GetAvailableSlotsResponse, error) {
	// This is a simplified implementation. In a real-world scenario, you would
	// calculate available slots based on existing events and working hours.
	s.logger.Info("GetAvailableSlots called via gRPC", "user_id", req.UserId)
	// Suppress unused context warning by using it in logger or ignoring
	_ = ctx

	return &pb.GetAvailableSlotsResponse{
		Slots:   []*pb.TimeSlot{},
		Success: true,
		Message: "Available slots retrieval not fully implemented",
	}, nil
}

// applyUpdateToEvent applies non-empty fields from req to the provided event and
// updates the UpdatedAt timestamp. Parsing errors for times are ignored (no-op).
func applyUpdateToEvent(event *models.Event, req *updateEventRequest) {
	if req.Title != "" {
		event.Title = req.Title
	}
	if req.Description != "" {
		event.Description = req.Description
	}
	if req.Location != "" {
		event.Location = req.Location
	}
	if req.IsRecurring {
		event.IsRecurring = req.IsRecurring
	}
	if req.RecurrenceRule != "" {
		event.RecurrenceRule = req.RecurrenceRule
	}
	if req.RecurrenceException != "" {
		event.RecurrenceException = req.RecurrenceException
	}
	if req.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, req.StartTime); err == nil {
			event.StartTime = t
		}
	}
	if req.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, req.EndTime); err == nil {
			event.EndTime = t
		}
	}
	event.UpdatedAt = time.Now()
}

type updateEventRequest struct {
	Title               string `json:"title,omitempty"`
	Description         string `json:"description,omitempty"`
	StartTime           string `json:"start_time,omitempty"`
	EndTime             string `json:"end_time,omitempty"`
	Location            string `json:"location,omitempty"`
	IsRecurring         bool   `json:"is_recurring,omitempty"`
	RecurrenceRule      string `json:"recurrence_rule,omitempty"`
	RecurrenceException string `json:"recurrence_exception,omitempty"`
}
