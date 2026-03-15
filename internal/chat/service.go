package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
	"github.com/waydxd/Orbit-core/pkg/metrics"
	pb "github.com/waydxd/Orbit-core/proto/calendar"
)

var (
	ErrInvalidIdempotencyKey = errors.New("invalid idempotency key")
	ErrActionNotPending      = errors.New("action not pending")
	ErrActionExpired         = errors.New("action has expired")
	ErrActionValidation      = errors.New("action validation failed")
	ErrActionConflict        = errors.New("action conflicts with existing data")
	ErrConversationNotFound  = errors.New("conversation not found")
	ErrConversationForbidden = errors.New("conversation does not belong to user")
)

const (
	maxChatRequestBytes int64 = 1 << 20 // 1MB
	agentRPCTimeout           = 15 * time.Second
	actionRPCTimeout          = 10 * time.Second
)

// GRPCClient is the interface for communicating with the remote orbi agent via gRPC.
type GRPCClient interface {
	HealthCheck(ctx context.Context) error
	ProcessMessage(ctx context.Context, req *pb.ProcessMessageRequest) (*pb.ProcessMessageResponse, error)
	GetCalendarServiceClient() pb.CalendarServiceClient
}

// Service represents the Chat Service for chatbot functionality
type Service struct {
	config            *config.Config
	logger            *logger.Logger
	repo              Repository
	grpcClient        GRPCClient
	policyValidator   *PolicyValidator
	actionExpiryHours int
}

// NewService creates a new Chat Service
func NewService(cfg *config.Config, log *logger.Logger, repo Repository, grpcClient GRPCClient) *Service {
	return &Service{
		config:            cfg,
		logger:            log,
		repo:              repo,
		grpcClient:        grpcClient,
		policyValidator:   NewPolicyValidator(),
		actionExpiryHours: 24, // Default 24 hours, can be made configurable via cfg
	}
}

// RegisterRoutes registers chat routes
func (s *Service) RegisterRoutes(router *mux.Router) {
	chatRouter := router.PathPrefix("/chat").Subrouter()

	chatRouter.HandleFunc("/health", s.handleHealth).Methods("GET")
	chatRouter.HandleFunc("/conversations", s.handleCreateConversation).Methods("POST")
	chatRouter.HandleFunc("/conversations/{conversation_id}", s.handleDeleteConversation).Methods("DELETE")
	chatRouter.HandleFunc("/messages", s.handlePostMessage).Methods("POST")
	chatRouter.HandleFunc("/conversations/{conversation_id}", s.handleGetConversation).Methods("GET")
	chatRouter.HandleFunc("/actions/{action_id}/confirm", s.handleConfirmAction).Methods("POST")
	chatRouter.HandleFunc("/actions/{action_id}/cancel", s.handleCancelAction).Methods("POST")
	chatRouter.HandleFunc("/actions/{action_id}", s.handleGetAction).Methods("GET")
	chatRouter.HandleFunc("/metrics", s.handleGetMetrics).Methods("GET")
}

// Request/Response types

type PostMessageRequest struct {
	Message        string                 `json:"message"`
	ConversationID string                 `json:"conversation_id,omitempty"`
	Context        map[string]interface{} `json:"context,omitempty"`
}

type PostMessageResponse struct {
	ConversationID        string                 `json:"conversation_id"`
	Reply                 string                 `json:"reply"`
	ProposedActionSummary string                 `json:"proposed_action_summary,omitempty"`
	ActionID              string                 `json:"action_id,omitempty"`
	CorrelationID         string                 `json:"correlation_id"`
	Metadata              map[string]interface{} `json:"metadata,omitempty"`
}

type ConversationResponse struct {
	ConversationID string                  `json:"conversation_id"`
	UserID         string                  `json:"user_id"`
	Messages       []*models.ChatMessage   `json:"messages"`
	PendingActions []*models.PendingAction `json:"pending_actions"`
	Status         string                  `json:"status"`
}

type CreateConversationResponse struct {
	ConversationID string `json:"conversation_id"`
	Status         string `json:"status"`
}

type ActionResponse struct {
	ActionID       string          `json:"action_id"`
	ActionType     string          `json:"action_type"`
	ProposedAction json.RawMessage `json:"proposed_action"`
	Status         string          `json:"status"`
	CreatedAt      time.Time       `json:"created_at"`
	ExpiresAt      time.Time       `json:"expires_at"`
}

type ConfirmActionRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
}

type ConfirmActionResponse struct {
	Success     bool                   `json:"success"`
	Message     string                 `json:"message"`
	Result      map[string]interface{} `json:"result,omitempty"`
	OperationID string                 `json:"operation_id,omitempty"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// ProposedAction represents an action proposed by the agent
type ProposedAction struct {
	ActionID   string                 `json:"action_id"`
	ActionType string                 `json:"action_type"`
	Action     map[string]interface{} `json:"action"`
	Summary    string                 `json:"summary"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

func decodeJSONStrict(w http.ResponseWriter, r *http.Request, dest interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxChatRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain exactly one JSON object")
	}
	return nil
}

func (s *Service) getConversationForUser(ctx context.Context, userID, conversationID string) (*models.Conversation, error) {
	conv, err := s.repo.GetConversationByID(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrConversationNotFound, conversationID)
	}
	if conv.UserID != userID {
		return nil, ErrConversationForbidden
	}
	if conv.Status == "deleted" {
		return nil, fmt.Errorf("%w: %s", ErrConversationNotFound, conversationID)
	}
	return conv, nil
}

func (s *Service) persistUserMessage(ctx context.Context, convID, userID, message string, context map[string]interface{}) error {
	userMsg := &models.ChatMessage{
		ConversationID: convID,
		UserID:         userID,
		Role:           "user",
		Content:        message,
	}

	if context != nil {
		metadata, err := MarshalJSON(context)
		if err == nil {
			userMsg.Metadata = metadata
		}
	}

	_, err := s.repo.CreateMessage(ctx, userMsg)
	if err != nil {
		return fmt.Errorf("failed to persist user message: %w", err)
	}

	return nil
}

func (s *Service) persistAssistantMessage(ctx context.Context, convID, userID, message string) error {
	assistantMsg := &models.ChatMessage{
		ConversationID: convID,
		UserID:         userID,
		Role:           "assistant",
		Content:        message,
	}
	_, err := s.repo.CreateMessage(ctx, assistantMsg)
	if err != nil {
		return fmt.Errorf("failed to persist agent reply: %w", err)
	}
	return nil
}

func (s *Service) handleValidationError(w http.ResponseWriter, err error) {
	m := metrics.GetInstance()
	switch {
	case errors.Is(err, ErrInvalidIdempotencyKey):
		s.respondError(w, http.StatusBadRequest, "invalid_idempotency_key", err.Error(), "")
		m.IncrementErrors()
	case errors.Is(err, ErrActionNotPending):
		s.respondError(w, http.StatusConflict, "action_not_pending", err.Error(), "")
		m.IncrementErrors()
	case errors.Is(err, ErrActionExpired):
		s.respondError(w, http.StatusGone, "action_expired", err.Error(), "")
		m.IncrementExpiredActions()
	case errors.Is(err, ErrActionValidation):
		s.respondError(w, http.StatusBadRequest, "validation_failed", err.Error(), "")
		m.IncrementValidationErrors()
		m.IncrementFailedActions()
	case errors.Is(err, ErrActionConflict):
		s.respondError(w, http.StatusConflict, "conflict_detected", err.Error(), "")
		m.IncrementConflictErrors()
	default:
		s.respondError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred during validation", err.Error())
		m.IncrementErrors()
	}
}

func (s *Service) validateActionForConfirmation(ctx context.Context, action *models.PendingAction, idempotencyKey string) error {
	if idempotencyKey != action.IdempotencyKey {
		return ErrInvalidIdempotencyKey
	}

	if action.Status != "pending" {
		return fmt.Errorf("%w: action is %s", ErrActionNotPending, action.Status)
	}

	if time.Now().After(action.ExpiresAt) {
		if err := s.repo.UpdatePendingActionStatus(ctx, action.ActionID, "expired", action.Version, "Action expired"); err != nil {
			s.logger.Error("Failed to update action status to expired", "error", err)
		}
		return ErrActionExpired
	}

	if err := s.policyValidator.ValidateAction(action.ActionType, action.ProposedAction); err != nil {
		s.logger.Error("Action failed validation on confirmation", "error", err, "action_id", action.ActionID)
		if uerr := s.repo.UpdatePendingActionStatus(ctx, action.ActionID, "failed", action.Version, fmt.Sprintf("Validation failed: %v", err)); uerr != nil {
			s.logger.Error("Failed to update action status to failed", "error", uerr)
		}
		return fmt.Errorf("%w: %v", ErrActionValidation, err)
	}

	if err := s.checkConflicts(ctx, action); err != nil {
		s.logger.Warn("Conflict detected", "error", err, "action_id", action.ActionID)
		return fmt.Errorf("%w: %v", ErrActionConflict, err)
	}

	return nil
}

func (s *Service) respondError(w http.ResponseWriter, statusCode int, code, message, details string) {
	w.WriteHeader(statusCode)
	response := ErrorResponse{
		Error:   message,
		Code:    code,
		Details: details,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode error response", "error", err)
	}
}

// CleanupExpiredActions is a background job to expire stale pending actions
func (s *Service) CleanupExpiredActions(ctx context.Context) error {
	s.logger.Info("Running cleanup job for expired actions")

	expiredActions, err := s.repo.GetExpiredActions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get expired actions: %w", err)
	}

	for _, action := range expiredActions {
		err := s.repo.UpdatePendingActionStatus(ctx, action.ActionID, "expired", action.Version, "Automatically expired due to TTL")
		if err != nil {
			s.logger.Error("Failed to expire action", "action_id", action.ActionID, "error", err)
		}
	}

	if len(expiredActions) > 0 {
		const maxIDsToLog = 20
		actionIDs := make([]string, 0, maxIDsToLog)
		for i, action := range expiredActions {
			if i < maxIDsToLog {
				actionIDs = append(actionIDs, action.ActionID)
			} else {
				break
			}
		}

		if len(expiredActions) <= maxIDsToLog {
			s.logger.Info("Expired actions", "count", len(expiredActions), "action_ids", actionIDs)
		} else {
			s.logger.Info("Expired actions", "count", len(expiredActions), "action_ids", actionIDs, "note", fmt.Sprintf("showing first %d of %d", maxIDsToLog, len(expiredActions)))
		}
	}

	s.logger.Info("Cleanup job completed", "expired_count", len(expiredActions))
	return nil
}

// checkConflicts checks for conflicts before executing an action
func (s *Service) checkConflicts(_ context.Context, action *models.PendingAction) error {
	if action.ActionType == "create_event" || action.ActionType == "update_event" {
		var eventData map[string]interface{}
		if err := json.Unmarshal(action.ProposedAction, &eventData); err != nil {
			return fmt.Errorf("failed to parse event data: %w", err)
		}

		startTime, endTime, hasStart, hasEnd := extractTimeFields(eventData)

		if hasStart && hasEnd {
			s.logger.Info("Conflict check",
				"action_id", action.ActionID,
				"start_time", time.Unix(startTime, 0),
				"end_time", time.Unix(endTime, 0),
			)
		}
	}

	return nil
}

// parseProposedAction retrieves pending actions created by the gRPC interceptor.
// When the agent calls CalendarService methods, the interceptor creates PendingAction records.
// We retrieve the most recent one for this conversation and correlation (if available) and return it.
func (s *Service) parseProposedAction(ctx context.Context, _ *pb.ProcessMessageResponse, conversationID, correlationID string) *ProposedAction {
	pendingActions, err := s.repo.GetPendingActionsByConversation(ctx, conversationID)
	if err != nil {
		s.logger.Error("Failed to get pending actions", "error", err, "conversation_id", conversationID)
		return nil
	}

	// Find the most recently created pending action, preferring those with a matching correlation ID.
	var latestMatchingAction *models.PendingAction
	var latestPendingAction *models.PendingAction
	for _, action := range pendingActions {
		if action.Status != "pending" {
			continue
		}

		// Track the most recent pending action overall as a fallback.
		if latestPendingAction == nil || action.CreatedAt.After(latestPendingAction.CreatedAt) {
			latestPendingAction = action
		}

		// Prefer actions whose correlation ID matches the provided correlationID.
		if action.CorrelationID == correlationID {
			if latestMatchingAction == nil || action.CreatedAt.After(latestMatchingAction.CreatedAt) {
				latestMatchingAction = action
			}
		}
	}

	// Prefer the newest correlation-matching action; fall back to newest pending action overall.
	latestAction := latestMatchingAction
	if latestAction == nil {
		latestAction = latestPendingAction
	}

	if latestAction == nil {
		return nil
	}

	// Parse the proposed action data
	var actionData map[string]interface{}
	if err := json.Unmarshal(latestAction.ProposedAction, &actionData); err != nil {
		s.logger.Error("Failed to unmarshal proposed action", "error", err, "action_id", latestAction.ActionID)
		return nil
	}

	// Generate a human-readable summary
	summary := generateActionSummary(latestAction.ActionType, actionData)

	return &ProposedAction{
		ActionID:   latestAction.ActionID,
		ActionType: latestAction.ActionType,
		Action:     actionData,
		Summary:    summary,
	}
}

// generateActionSummary creates a human-readable summary of the action
func generateActionSummary(actionType string, actionData map[string]interface{}) string {
	switch actionType {
	case "create_event":
		title, _ := actionData["title"].(string)
		location, _ := actionData["location"].(string)
		if location != "" {
			return fmt.Sprintf("Create event: %s at %s", title, location)
		}
		return fmt.Sprintf("Create event: %s", title)
	case "update_event":
		title, _ := actionData["title"].(string)
		return fmt.Sprintf("Update event: %s", title)
	case "delete_event":
		eventID, _ := actionData["id"].(string)
		title, _ := actionData["title"].(string)
		if title != "" {
			return fmt.Sprintf("Delete event: %s", title)
		}
		return fmt.Sprintf("Delete event: %s", eventID)
	default:
		return fmt.Sprintf("Execute %s", actionType)
	}
}

// extractTimeFields is a helper to extract and validate time fields from action data
func extractTimeFields(actionData map[string]interface{}) (startTime int64, endTime int64, hasStart bool, hasEnd bool) {
	startFloat, hasStart := actionData["start_time"].(float64)
	endFloat, hasEnd := actionData["end_time"].(float64)

	if hasStart {
		startTime = int64(startFloat)
		// Validate timestamp is reasonable (not negative, not too far in future)
		if startTime < 0 {
			startTime = 0
			hasStart = false
		} else if startTime > time.Now().AddDate(10, 0, 0).Unix() {
			// Reject timestamps more than 10 years in the future
			startTime = 0
			hasStart = false
		}
	}
	if hasEnd {
		endTime = int64(endFloat)
		// Validate timestamp is reasonable (not negative, not too far in future)
		if endTime < 0 {
			endTime = 0
			hasEnd = false
		} else if endTime > time.Now().AddDate(10, 0, 0).Unix() {
			// Reject timestamps more than 10 years in the future
			endTime = 0
			hasEnd = false
		}
	}

	return startTime, endTime, hasStart, hasEnd
}
