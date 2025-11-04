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
	pb "github.com/waydxd/Orbit-core/proto/calendar"
)

// Service represents the Calendar & Task Service
type Service struct {
	pb.UnimplementedCalendarDataServiceServer
	config    *config.Config
	logger    *logger.Logger
	eventRepo EventRepository
	taskRepo  TaskRepository
}

// NewService creates a new Calendar Service
func NewService(cfg *config.Config, log *logger.Logger, eventRepo EventRepository, taskRepo TaskRepository) *Service {
	return &Service{
		config:    cfg,
		logger:    log,
		eventRepo: eventRepo,
		taskRepo:  taskRepo,
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

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "user_id required"})
		return
	}

	startTimeStr := r.URL.Query().Get("start_time")
	endTimeStr := r.URL.Query().Get("end_time")

	startTime := time.Now()
	endTime := time.Now().AddDate(0, 0, 30)

	if startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = t
		}
	}
	if endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			endTime = t
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	events, err := s.eventRepo.ListEvents(ctx, userID, startTime, endTime)
	if err != nil {
		s.logger.Error("failed to list events", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to list events"})
		return
	}

	json.NewEncoder(w).Encode(events)
}

// createEvent creates a new event
func (s *Service) createEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		UserID      string `json:"user_id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		StartTime   string `json:"start_time"`
		EndTime     string `json:"end_time"`
		Location    string `json:"location"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid start_time format"})
		return
	}

	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid end_time format"})
		return
	}

	event := &models.Event{
		ID:          uuid.New().String(),
		UserID:      req.UserID,
		Title:       req.Title,
		Description: req.Description,
		StartTime:   startTime,
		EndTime:     endTime,
		Location:    req.Location,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := s.eventRepo.CreateEvent(ctx, event); err != nil {
		s.logger.Error("failed to create event", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to create event"})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(event)
}

// getEvent retrieves an event by ID
func (s *Service) getEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	id := mux.Vars(r)["id"]
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	event, err := s.eventRepo.GetEventByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get event", "err", err)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "event not found"})
		return
	}

	json.NewEncoder(w).Encode(event)
}

// updateEvent updates an existing event
func (s *Service) updateEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	id := mux.Vars(r)["id"]

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		StartTime   string `json:"start_time"`
		EndTime     string `json:"end_time"`
		Location    string `json:"location"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	event, err := s.eventRepo.GetEventByID(ctx, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "event not found"})
		return
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

	if err := s.eventRepo.UpdateEvent(ctx, event); err != nil {
		s.logger.Error("failed to update event", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to update event"})
		return
	}

	json.NewEncoder(w).Encode(event)
}

// deleteEvent deletes an event
func (s *Service) deleteEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	id := mux.Vars(r)["id"]
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := s.eventRepo.DeleteEvent(ctx, id); err != nil {
		s.logger.Error("failed to delete event", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to delete event"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ===== Task HTTP Handlers =====

// listTasks retrieves tasks for a user
func (s *Service) listTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "user_id required"})
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
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to list tasks"})
		return
	}

	json.NewEncoder(w).Encode(tasks)
}

// createTask creates a new task
func (s *Service) createTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		UserID      string `json:"user_id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		DueDate     string `json:"due_date"`
		Priority    string `json:"priority"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
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
		UserID:      req.UserID,
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
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to create task"})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

// getTask retrieves a task by ID
func (s *Service) getTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	id := mux.Vars(r)["id"]
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	task, err := s.taskRepo.GetTaskByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get task", "err", err)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "task not found"})
		return
	}

	json.NewEncoder(w).Encode(task)
}

// updateTask updates an existing task
func (s *Service) updateTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	id := mux.Vars(r)["id"]

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		DueDate     string `json:"due_date"`
		Completed   *bool  `json:"completed"`
		Priority    string `json:"priority"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	task, err := s.taskRepo.GetTaskByID(ctx, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "task not found"})
		return
	}

	if req.Title != "" {
		task.Title = req.Title
	}
	if req.Description != "" {
		task.Description = req.Description
	}
	if req.Priority != "" {
		task.Priority = req.Priority
	}
	if req.DueDate != "" {
		if t, err := time.Parse(time.RFC3339, req.DueDate); err == nil {
			task.DueDate = t
		}
	}
	if req.Completed != nil {
		task.Completed = *req.Completed
	}

	if err := s.taskRepo.UpdateTask(ctx, task); err != nil {
		s.logger.Error("failed to update task", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to update task"})
		return
	}

	json.NewEncoder(w).Encode(task)
}

// deleteTask deletes a task
func (s *Service) deleteTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	id := mux.Vars(r)["id"]
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := s.taskRepo.DeleteTask(ctx, id); err != nil {
		s.logger.Error("failed to delete task", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to delete task"})
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

	return &pb.QueryEventsResponse{
		Events:     pbEvents,
		TotalCount: int32(len(pbEvents)),
		Success:    true,
		Message:    "Query executed successfully",
	}, nil
}

// Adapter methods to satisfy agent.CalendarServiceInterface and gateway.CalendarServiceInterface

// ListEvents returns events across users (userID omitted) or can be extended to filter by status.
func (s *Service) ListEvents(ctx context.Context, startTime, endTime int64, status string) ([]interface{}, error) {
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

// CreateEvent accepts flexible payloads (map[string]interface{}, *models.Event, pb.Event) and creates an event
func (s *Service) CreateEvent(ctx context.Context, event interface{}) (interface{}, error) {
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
		if st, ok := v["start_time"].(int64); ok {
			ev.StartTime = time.Unix(st, 0)
		} else if stf, ok := v["start_time"].(float64); ok {
			ev.StartTime = time.Unix(int64(stf), 0)
		} else if sts, ok := v["start_time"].(string); ok {
			if t, err := time.Parse(time.RFC3339, sts); err == nil {
				ev.StartTime = t
			}
		}
		if et, ok := v["end_time"].(int64); ok {
			ev.EndTime = time.Unix(et, 0)
		} else if etf, ok := v["end_time"].(float64); ok {
			ev.EndTime = time.Unix(int64(etf), 0)
		} else if ets, ok := v["end_time"].(string); ok {
			if t, err := time.Parse(time.RFC3339, ets); err == nil {
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
		return nil, fmt.Errorf("unsupported event type")
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
	return &ev, nil
}

// UpdateEvent updates an event by id using flexible payloads
func (s *Service) UpdateEvent(ctx context.Context, id string, event interface{}) (interface{}, error) {
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

// DeleteEvent deletes an event by id
func (s *Service) DeleteEvent(ctx context.Context, id string) error {
	if err := s.eventRepo.DeleteEvent(ctx, id); err != nil {
		s.logger.Error("failed to delete event (adapter)", "id", id, "err", err)
		return err
	}
	return nil
}
