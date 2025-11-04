package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	pb "github.com/waydxd/Orbit-Orbi/proto/calendar"
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
	ListEvents(ctx context.Context, startTime, endTime int64, status string) ([]interface{}, error)
	CreateEvent(ctx context.Context, event interface{}) (interface{}, error)
	UpdateEvent(ctx context.Context, id string, event interface{}) (interface{}, error)
	DeleteEvent(ctx context.Context, id string) error
}

// PromptRequest represents the user prompt for the AI agent
type PromptRequest struct {
	Prompt    string `json:"prompt"`
	StartTime int64  `json:"start_time,omitempty"`  // Unix timestamp
	EndTime   int64  `json:"end_time,omitempty"`    // Unix timestamp
	Context   string `json:"context,omitempty"`     // Additional context
}

// AgentResponse represents the AI agent's response
type AgentResponse struct {
	Success        bool        `json:"success"`
	Message        string      `json:"message"`
	Action         string      `json:"action,omitempty"`        // e.g., "create_event", "get_events"
	Data           interface{} `json:"data,omitempty"`          // Response data
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
	events, err := s.calendarSvc.ListEvents(ctx, startTime, endTime, "")
	if err != nil {
		s.logger.Error("Failed to fetch events", "error", err)
		events = []interface{}{}
	}

	// Get available slots from the Orbi agent
	availableSlots, err := s.getAvailableSlots(ctx, startTime, endTime, 3600) // 1 hour duration
	if err != nil {
		s.logger.Warn("Failed to get available slots", "error", err)
	}

	// Prepare response with calendar context
	resp := AgentResponse{
		Success:        true,
		Message:        "Prompt processed successfully",
		Timestamp:      time.Now().Unix(),
		CalendarEvents: events,
		Data: map[string]interface{}{
			"available_slots": availableSlots,
			"prompt":          req.Prompt,
		},
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Error("Failed to encode response", "error", err)
	}
}

// getAvailableSlots retrieves available time slots from the Orbi agent
func (s *Service) getAvailableSlots(ctx context.Context, startTime, endTime, duration int64) (interface{}, error) {
	// Create gRPC client for calendar service
	client := s.grpcClient.GetCalendarServiceClient()

	// Call GetAvailableSlots RPC
	res, err := client.GetAvailableSlots(ctx, &pb.GetAvailableSlotsRequest{
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  duration,
	})

	if err != nil {
		s.logger.Error("GetAvailableSlots RPC failed", "error", err)
		return nil, err
	}

	// Convert TimeSlot messages to JSON-serializable format
	slots := make([]map[string]interface{}, len(res.Slots))
	for i, slot := range res.Slots {
		slots[i] = map[string]interface{}{
			"start_time": slot.StartTime,
			"end_time":   slot.EndTime,
		}
	}

	return map[string]interface{}{
		"slots":   slots,
		"success": res.Success,
		"message": res.Message,
	}, nil
}

// healthCheck returns the health status of the agent service
func (s *Service) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check gRPC connection
	err := s.grpcClient.HealthCheck(r.Context())
	if err != nil {
		s.logger.Error("Agent service health check failed", "error", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "healthy",
	})
}
