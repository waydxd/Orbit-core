package calendar

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
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

// Event handlers
func (s *Service) listEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// TODO: Fetch events from PostgreSQL
	events := []models.Event{}

	json.NewEncoder(w).Encode(events)
}

func (s *Service) createEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	// TODO: Store event in PostgreSQL
	s.logger.Info("Event created", "title", event.Title)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(event)
}

func (s *Service) getEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	eventID := vars["id"]

	// TODO: Fetch event from PostgreSQL
	s.logger.Info("Fetching event", "id", eventID)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"id": eventID})
}

func (s *Service) updateEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	eventID := vars["id"]

	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	// TODO: Update event in PostgreSQL
	s.logger.Info("Event updated", "id", eventID)

	json.NewEncoder(w).Encode(event)
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
func (s *Service) listTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// TODO: Fetch tasks from PostgreSQL
	tasks := []models.Task{}

	json.NewEncoder(w).Encode(tasks)
}

func (s *Service) createTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var task models.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	// TODO: Store task in PostgreSQL
	s.logger.Info("Task created", "title", task.Title)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

func (s *Service) getTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	taskID := vars["id"]

	// TODO: Fetch task from PostgreSQL
	s.logger.Info("Fetching task", "id", taskID)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"id": taskID})
}

func (s *Service) updateTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	taskID := vars["id"]

	var task models.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	// TODO: Update task in PostgreSQL
	s.logger.Info("Task updated", "id", taskID)

	json.NewEncoder(w).Encode(task)
}

func (s *Service) deleteTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	taskID := vars["id"]

	// TODO: Delete task from PostgreSQL
	s.logger.Info("Task deleted", "id", taskID)

	w.WriteHeader(http.StatusNoContent)
}
