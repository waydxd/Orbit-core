package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/grpc"
	"github.com/waydxd/Orbit-core/pkg/logger"
)

// Service represents the Agent Service for AI interactions
type Service struct {
	config      *config.Config
	logger      *logger.Logger
	grpcClient  *grpc.CalendarGRPCClient
	calendarSvc CalendarServiceInterface
}

// CalendarServiceInterface defines calendar operations
type CalendarServiceInterface interface {
	ListEventsAdapter(ctx context.Context, startTime, endTime int64, status string) ([]interface{}, error)
	CreateEventAdapter(ctx context.Context, event interface{}) (interface{}, error)
	UpdateEventAdapter(ctx context.Context, id string, event interface{}) (interface{}, error)
	DeleteEventAdapter(ctx context.Context, id string) error
}

// PromptRequest represents the user prompt for the AI agent
type PromptRequest struct {
	Prompt    string `json:"prompt"`
	StartTime int64  `json:"start_time,omitempty"` // Unix timestamp
	EndTime   int64  `json:"end_time,omitempty"`   // Unix timestamp
	Context   string `json:"context,omitempty"`    // Additional context
}

// AgentResponse represents the AI agent's response
type AgentResponse struct {
	Success        bool        `json:"success"`
	Message        string      `json:"message"`
	Action         string      `json:"action,omitempty"`          // e.g., "create_event", "get_events"
	Data           interface{} `json:"data,omitempty"`            // Response data
	CalendarEvents interface{} `json:"calendar_events,omitempty"` // Context events sent to agent
	Timestamp      int64       `json:"timestamp"`
}

// NewService creates a new Agent Service
func NewService(cfg *config.Config, log *logger.Logger, grpcClient *grpc.CalendarGRPCClient, calendarSvc CalendarServiceInterface) *Service {
	return &Service{
		config:      cfg,
		logger:      log,
		grpcClient:  grpcClient,
		calendarSvc: calendarSvc,
	}
}

// RegisterRoutes registers agent routes
func (s *Service) RegisterRoutes(router *mux.Router) {
	agentRouter := router.PathPrefix("/agent").Subrouter()

	agentRouter.HandleFunc("/chat", s.handleChat).Methods("POST")
	agentRouter.HandleFunc("/health", s.healthCheck).Methods("GET")
}

// handleChat processes user prompts and communicates with the AI agent
func (s *Service) handleChat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req PromptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("Invalid request body", "error", err)
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Prompt == "" {
		http.Error(w, `{"error":"Prompt is required"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Set default time range if not provided
	now := time.Now()
	startTime := req.StartTime
	endTime := req.EndTime

	if startTime == 0 {
		startTime = now.Unix()
	}
	if endTime == 0 {
		endTime = now.AddDate(0, 0, 7).Unix() // Default to next 7 days
	}

	// Fetch calendar events to provide context to the agent
	events, err := s.calendarSvc.ListEventsAdapter(ctx, startTime, endTime, "")
	if err != nil {
		s.logger.Error("Failed to fetch events", "error", err)
		events = []interface{}{}
	}

	// TODO: The orbit-orbi external agent should call CalendarService methods on orbit-core
	// This orbit-core agent service should not be calling CalendarService methods
	// For now, we just return calendar events without calling the external agent

	// Prepare response with calendar context
	resp := AgentResponse{
		Success:        true,
		Message:        "Prompt processed successfully (Note: External agent integration needed)",
		Timestamp:      time.Now().Unix(),
		CalendarEvents: events,
		Data: map[string]interface{}{
			"prompt": req.Prompt,
		},
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Error("Failed to encode response", "error", err)
	}
}

// healthCheck returns the health status of the agent service
func (s *Service) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check gRPC connection
	err := s.grpcClient.HealthCheck(r.Context())
	if err != nil {
		s.logger.Error("Agent service health check failed", "error", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}); err != nil {
			s.logger.Error("failed to write healthCheck error response", "error", err)
			return
		}
		return
	}

	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "healthy",
	}); err != nil {
		s.logger.Error("failed to write healthCheck success response", "error", err)
		return
	}
}
