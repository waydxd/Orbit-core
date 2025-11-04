package calendar

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	pb "github.com/waydxd/Orbit-Orbi/proto/calendar"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
)

// Service represents the Calendar & Task Service
type Service struct {
	config *config.Config
	logger *logger.Logger
}

// NewService creates a new Calendar Service
func NewService(cfg *config.Config, log *logger.Logger) *Service {
	return &Service{
		config: cfg,
		logger: log,
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

// ==== gRPC Server Implementation for CalendarDataService ====
// This allows Agent to call Core and fetch calendar data

// GetCalendarData implements CalendarDataService.GetCalendarData
// Called by Agent to retrieve calendar events for a user
func (s *Service) GetCalendarData(ctx context.Context, req *pb.GetCalendarDataRequest) (*pb.GetCalendarDataResponse, error) {
	s.logger.Info("GetCalendarData called by Agent", "user_id", req.UserId, "start_time", req.StartTime, "end_time", req.EndTime)

	// TODO: Fetch events from database based on user_id and time range
	// For now, returning empty list
	events := []*pb.Event{}

	return &pb.GetCalendarDataResponse{
		Events:  events,
		Success: true,
		Message: "Calendar data retrieved successfully",
	}, nil
}

// GetUserAvailability implements CalendarDataService.GetUserAvailability
// Called by Agent to check if a user is available during a specific time slot
func (s *Service) GetUserAvailability(ctx context.Context, req *pb.GetUserAvailabilityRequest) (*pb.GetUserAvailabilityResponse, error) {
	s.logger.Info("GetUserAvailability called by Agent", "user_id", req.UserId, "start_time", req.StartTime, "end_time", req.EndTime)

	// TODO: Check for conflicts in database
	// For now, assuming user is available
	status := &pb.AvailabilityStatus{
		Available:          true,
		Reason:             "No conflicts found",
		ConflictingEvents:  []*pb.Event{},
	}

	return &pb.GetUserAvailabilityResponse{
		Status:  status,
		Success: true,
		Message: "Availability check completed",
	}, nil
}

// QueryEvents implements CalendarDataService.QueryEvents
// Called by Agent to perform complex queries on calendar events
func (s *Service) QueryEvents(ctx context.Context, req *pb.QueryEventsRequest) (*pb.QueryEventsResponse, error) {
	s.logger.Info("QueryEvents called by Agent", "user_id", req.UserId, "query", req.Query)

	// TODO: Implement query logic to search for events matching the query string
	// For now, returning empty list
	events := []*pb.Event{}

	return &pb.QueryEventsResponse{
		Events:     events,
		TotalCount: 0,
		Success:    true,
		Message:    "Query executed successfully",
	}, nil
}

// Event handlers
func (s *Service) listEvents(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = w

	// TODO: Fetch events from PostgreSQL
	var events []models.Event

	if err := json.NewEncoder(w).Encode(events); err != nil {
		s.logger.Error("Error encoding listEvents response", "err", err)
		return
	}
}

func (s *Service) createEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		s.logger.Error("invalid createEvent request", "err", err)
		if encErr := json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"}); encErr != nil {
			s.logger.Error("Error encoding createEvent error response", "err", encErr)
		}
		return
	}

	// TODO: Store event in PostgreSQL
	s.logger.Info("Event created", "title", event.Title)

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(event); err != nil {
		s.logger.Error("Error encoding createEvent response", "err", err)
		return
	}
}

func (s *Service) getEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	eventID := vars["id"]

	// TODO: Fetch event from PostgreSQL
	s.logger.Info("Fetching event", "id", eventID)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"id": eventID}); err != nil {
		s.logger.Error("Error encoding getEvent response", "err", err)
		return
	}
}

func (s *Service) updateEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	eventID := vars["id"]

	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		s.logger.Error("invalid updateEvent request", "err", err)
		if encErr := json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"}); encErr != nil {
			s.logger.Error("Error encoding updateEvent error response", "err", encErr)
		}
		return
	}

	// TODO: Update event in PostgreSQL
	s.logger.Info("Event updated", "id", eventID)

	if err := json.NewEncoder(w).Encode(event); err != nil {
		s.logger.Error("Error encoding updateEvent response", "err", err)
		return
	}
}

func (s *Service) deleteEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	eventID := vars["id"]

	// TODO: Delete event from PostgreSQL
	s.logger.Info("Event deleted", "id", eventID)

	w.WriteHeader(http.StatusNoContent)
}

// Task handlers
func (s *Service) listTasks(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = w

	// TODO: Fetch tasks from PostgreSQL
	var tasks []models.Task

	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		s.logger.Error("Error encoding listTasks response", "err", err)
		return
	}
}

func (s *Service) createTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var task models.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		s.logger.Error("invalid createTask request", "err", err)
		if encErr := json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"}); encErr != nil {
			s.logger.Error("Error encoding createTask error response", "err", encErr)
		}
		return
	}

	// TODO: Store task in PostgreSQL
	s.logger.Info("Task created", "title", task.Title)

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(task); err != nil {
		s.logger.Error("Error encoding createTask response", "err", err)
		return
	}
}

func (s *Service) getTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	taskID := vars["id"]

	// TODO: Fetch task from PostgreSQL
	s.logger.Info("Fetching task", "id", taskID)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"id": taskID}); err != nil {
		s.logger.Error("Error encoding getTask response", "err", err)
		return
	}
}

func (s *Service) updateTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	taskID := vars["id"]

	var task models.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		s.logger.Error("invalid updateTask request", "err", err)
		if encErr := json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"}); encErr != nil {
			s.logger.Error("Error encoding updateTask error response", "err", encErr)
		}
		return
	}

	// TODO: Update task in PostgreSQL
	s.logger.Info("Task updated", "id", taskID)

	if err := json.NewEncoder(w).Encode(task); err != nil {
		s.logger.Error("Error encoding updateTask response", "err", err)
		return
	}
}

func (s *Service) deleteTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	taskID := vars["id"]

	// TODO: Delete task from PostgreSQL
	s.logger.Info("Task deleted", "id", taskID)

	w.WriteHeader(http.StatusNoContent)
}
