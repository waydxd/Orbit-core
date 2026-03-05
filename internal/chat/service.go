package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/grpc"
	"github.com/waydxd/Orbit-core/pkg/logger"
	"github.com/waydxd/Orbit-core/pkg/metrics"
	"github.com/waydxd/Orbit-core/pkg/middleware"
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

// Service represents the Chat Service for chatbot functionality
type Service struct {
	config            *config.Config
	logger            *logger.Logger
	repo              Repository
	grpcClient        *grpc.CalendarGRPCClient
	policyValidator   *PolicyValidator
	actionExpiryHours int
}

// NewService creates a new Chat Service
func NewService(cfg *config.Config, log *logger.Logger, repo Repository, grpcClient *grpc.CalendarGRPCClient) *Service {
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

// GET /chat/health - Get chat service health
func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := s.grpcClient.HealthCheck(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "healthy",
	})
}

// POST /chat/conversations - Create conversation
func (s *Service) handleCreateConversation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(ctx)
	if userID == "" {
		s.respondError(w, http.StatusUnauthorized, "unauthorized", "User ID not found in context", "")
		return
	}

	correlationID := GenerateCorrelationID()
	conv, err := s.repo.CreateConversation(ctx, userID, correlationID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "conversation_create_failed", "Failed to create conversation", err.Error())
		metrics.GetInstance().IncrementErrors()
		return
	}
	metrics.GetInstance().IncrementConversations()

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(CreateConversationResponse{
		ConversationID: conv.ID,
		Status:         conv.Status,
	}); err != nil {
		s.logger.Error("Failed to encode create conversation response", "error", err)
	}
}

// DELETE /chat/conversations/{conversation_id} - Soft delete conversation
func (s *Service) handleDeleteConversation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(ctx)
	if userID == "" {
		s.respondError(w, http.StatusUnauthorized, "unauthorized", "User ID not found in context", "")
		return
	}

	conversationID := mux.Vars(r)["conversation_id"]
	if conversationID == "" {
		s.respondError(w, http.StatusBadRequest, "missing_conversation_id", "Conversation ID is required", "")
		return
	}
	if _, err := uuid.Parse(conversationID); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid_conversation_id", "Invalid conversation ID format", "")
		return
	}

	conv, err := s.getConversationForUser(ctx, userID, conversationID)
	if err != nil {
		statusCode := http.StatusInternalServerError
		code := "conversation_error"
		if errors.Is(err, ErrConversationNotFound) {
			statusCode = http.StatusNotFound
			code = "conversation_not_found"
		}
		s.respondError(w, statusCode, code, "Failed to delete conversation", err.Error())
		metrics.GetInstance().IncrementErrors()
		return
	}

	if conv.Status == "deleted" {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Conversation already deleted"})
		return
	}

	if err := s.repo.UpdateConversationStatus(ctx, conversationID, "deleted"); err != nil {
		s.respondError(w, http.StatusInternalServerError, "delete_failed", "Failed to delete conversation", err.Error())
		metrics.GetInstance().IncrementErrors()
		return
	}

	if err := json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Conversation deleted"}); err != nil {
		s.logger.Error("Failed to encode delete conversation response", "error", err)
	}
}

// POST /chat/messages - Process user message
func (s *Service) handlePostMessage(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(ctx)
	if userID == "" {
		s.respondError(w, http.StatusUnauthorized, "unauthorized", "User ID not found in context", "")
		return
	}

	m := metrics.GetInstance()
	m.IncrementMessages()

	var req PostMessageRequest
	if err := decodeJSONStrict(w, r, &req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body", err.Error())
		m.IncrementErrors()
		return
	}

	if req.ConversationID == "" {
		s.respondError(w, http.StatusBadRequest, "missing_conversation_id", "Conversation ID is required", "")
		m.IncrementErrors()
		return
	}
	if _, err := uuid.Parse(req.ConversationID); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid_conversation_id", "Invalid conversation ID format", "")
		m.IncrementErrors()
		return
	}

	if req.Message == "" {
		s.respondError(w, http.StatusBadRequest, "missing_message", "Message is required", "")
		m.IncrementErrors()
		return
	}

	// Generate correlation ID
	correlationID := GenerateCorrelationID()
	s.logger.Info("Processing chat message", "correlation_id", correlationID, "user_id", userID)

	conv, err := s.getConversationForUser(ctx, userID, req.ConversationID)
	if err != nil {
		switch {
		case errors.Is(err, ErrConversationNotFound):
			s.respondError(w, http.StatusNotFound, "conversation_not_found", "Conversation not found", err.Error())
		case errors.Is(err, ErrConversationForbidden):
			s.respondError(w, http.StatusForbidden, "forbidden", "You do not have permission to access this conversation", "")
		default:
			s.respondError(w, http.StatusInternalServerError, "conversation_error", "Failed to get conversation", err.Error())
		}
		m.IncrementErrors()
		return
	}

	// Persist user message
	if err := s.persistUserMessage(ctx, conv.ID, userID, req.Message, req.Context); err != nil {
		s.respondError(w, http.StatusInternalServerError, "message_persist_failed", "Failed to persist user message", err.Error())
		m.IncrementErrors()
		return
	}

	// Forward to Agent Runner via gRPC
	agentReply, proposedAction, err := s.forwardToAgent(ctx, userID, req.Message, correlationID, conv.ID)
	if err != nil {
		s.logger.Error("Agent communication failed", "error", err, "correlation_id", correlationID)
		m.IncrementErrors()
		s.respondError(w, http.StatusBadGateway, "agent_error", "Agent communication failed", err.Error())
		return
	}

	// Persist agent reply
	if err := s.persistAssistantMessage(ctx, conv.ID, userID, agentReply); err != nil {
		s.logger.Error("Failed to persist assistant message", "error", err, "correlation_id", correlationID)
		m.IncrementErrors()
	}

	response := PostMessageResponse{
		ConversationID: conv.ID,
		Reply:          agentReply,
		CorrelationID:  correlationID,
	}

	// If the interceptor created an action, include it in response.
	if proposedAction != nil {
		response.ActionID = proposedAction.ActionID
		response.ProposedActionSummary = proposedAction.Summary
	}

	// Record latency
	m.RecordMessageLatency(time.Since(startTime))

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode response", "error", err)
	}
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

// GET /chat/conversations/{conversation_id} - Get conversation history
func (s *Service) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(ctx)
	if userID == "" {
		s.respondError(w, http.StatusUnauthorized, "unauthorized", "User ID not found in context", "")
		return
	}

	vars := mux.Vars(r)
	conversationID := vars["conversation_id"]

	if conversationID == "" {
		s.respondError(w, http.StatusBadRequest, "missing_conversation_id", "Conversation ID is required", "")
		return
	}
	if _, err := uuid.Parse(conversationID); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid_conversation_id", "Invalid conversation ID format", "")
		return
	}

	// Get conversation
	conv, err := s.getConversationForUser(ctx, userID, conversationID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "conversation_not_found", "Conversation not found", err.Error())
		return
	}

	// Get messages
	messages, err := s.repo.GetMessagesByConversation(ctx, conversationID)
	if err != nil {
		s.logger.Error("Failed to get messages", "error", err)
		messages = []*models.ChatMessage{}
	}

	// Get pending actions
	pendingActions, err := s.repo.GetPendingActionsByConversation(ctx, conversationID)
	if err != nil {
		s.logger.Error("Failed to get pending actions", "error", err)
		pendingActions = []*models.PendingAction{}
	}

	response := ConversationResponse{
		ConversationID: conv.ID,
		UserID:         conv.UserID,
		Messages:       messages,
		PendingActions: pendingActions,
		Status:         conv.Status,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode response", "error", err)
	}
}

// POST /chat/actions/{action_id}/confirm - Confirm and execute action
func (s *Service) handleConfirmAction(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(ctx)
	if userID == "" {
		s.respondError(w, http.StatusUnauthorized, "unauthorized", "User ID not found in context", "")
		return
	}

	m := metrics.GetInstance()

	vars := mux.Vars(r)
	actionID := vars["action_id"]
	if actionID == "" {
		s.respondError(w, http.StatusBadRequest, "missing_action_id", "Action ID is required", "")
		m.IncrementErrors()
		return
	}

	var req ConfirmActionRequest
	if err := decodeJSONStrict(w, r, &req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body", err.Error())
		m.IncrementErrors()
		return
	}

	action, err := s.repo.GetPendingActionByID(ctx, actionID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "action_not_found", "Action not found", err.Error())
		m.IncrementErrors()
		return
	}

	if action.UserID != userID {
		s.respondError(w, http.StatusForbidden, "forbidden", "You do not have permission to confirm this action", "")
		m.IncrementErrors()
		return
	}

	if err := s.validateActionForConfirmation(ctx, action, req.IdempotencyKey); err != nil {
		s.handleValidationError(w, err)
		return
	}

	result, operationID, err := s.executeAction(ctx, action)
	if err != nil {
		s.logger.Error("Failed to execute action", "error", err, "action_id", actionID)
		if err := s.repo.UpdatePendingActionStatus(ctx, actionID, "failed", action.Version, err.Error()); err != nil {
			s.logger.Error("Failed to update action status to failed", "error", err)
		}
		s.respondError(w, http.StatusInternalServerError, "execution_failed", "Failed to execute action", err.Error())
		m.IncrementFailedActions()
		return
	}

	if err := s.repo.UpdatePendingActionStatus(ctx, actionID, "confirmed", action.Version, ""); err != nil {
		s.logger.Error("Failed to update action status", "error", err)
		// Don't fail the request here as the action was successful
	}

	m.IncrementConfirmedActions()
	m.RecordActionLatency(time.Since(startTime))

	response := ConfirmActionResponse{
		Success:     true,
		Message:     "Action executed successfully",
		Result:      result,
		OperationID: operationID,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode response", "error", err)
	}
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

// POST /chat/actions/{action_id}/cancel - Cancel pending action
func (s *Service) handleCancelAction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(ctx)
	if userID == "" {
		s.respondError(w, http.StatusUnauthorized, "unauthorized", "User ID not found in context", "")
		return
	}

	m := metrics.GetInstance()

	vars := mux.Vars(r)
	actionID := vars["action_id"]
	if actionID == "" {
		s.respondError(w, http.StatusBadRequest, "missing_action_id", "Action ID is required", "")
		m.IncrementErrors()
		return
	}

	// Get pending action
	action, err := s.repo.GetPendingActionByID(ctx, actionID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "action_not_found", "Action not found", err.Error())
		m.IncrementErrors()
		return
	}

	if action.UserID != userID {
		s.respondError(w, http.StatusForbidden, "forbidden", "You do not have permission to cancel this action", "")
		m.IncrementErrors()
		return
	}

	// Check if action can be canceled
	if action.Status != "pending" {
		s.respondError(w, http.StatusConflict, "action_not_pending", fmt.Sprintf("Action is %s and cannot be canceled", action.Status), "")
		m.IncrementErrors()
		return
	}

	// Update action status to canceled
	err = s.repo.UpdatePendingActionStatus(ctx, actionID, "cancelled", action.Version, "Cancelled by user") //nolint: misspell
	if err != nil {
		s.logger.Error("Failed to cancel action", "error", err)
		s.respondError(w, http.StatusInternalServerError, "cancel_failed", "Failed to cancel action", err.Error())
		m.IncrementErrors()
		return
	}

	s.logger.Info("Action canceled", "action_id", actionID, "user_id", action.UserID)
	m.IncrementCancelledActions()

	response := map[string]interface{}{
		"success": true,
		"message": "Action canceled successfully",
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode response", "error", err)
	}
}

// GET /chat/actions/{action_id} - Get action details
func (s *Service) handleGetAction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(ctx)
	if userID == "" {
		s.respondError(w, http.StatusUnauthorized, "unauthorized", "User ID not found in context", "")
		return
	}

	vars := mux.Vars(r)
	actionID := vars["action_id"]
	if actionID == "" {
		s.respondError(w, http.StatusBadRequest, "missing_action_id", "Action ID is required", "")
		return
	}

	// Get pending action
	action, err := s.repo.GetPendingActionByID(ctx, actionID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "action_not_found", "Action not found", err.Error())
		return
	}

	if action.UserID != userID {
		s.respondError(w, http.StatusForbidden, "forbidden", "You do not have permission to access this action", "")
		return
	}

	response := ActionResponse{
		ActionID:       action.ActionID,
		ActionType:     action.ActionType,
		ProposedAction: action.ProposedAction,
		Status:         action.Status,
		CreatedAt:      action.CreatedAt,
		ExpiresAt:      action.ExpiresAt,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode response", "error", err)
	}
}

// Helper functions

type ProposedAction struct {
	ActionID   string                 `json:"action_id"`
	ActionType string                 `json:"action_type"`
	Action     map[string]interface{} `json:"action"`
	Summary    string                 `json:"summary"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

func (s *Service) forwardToAgent(ctx context.Context, userID, message, correlationID, conversationID string) (string, *ProposedAction, error) {
	s.logger.Info("Forwarding to agent", "correlation_id", correlationID, "user_id", userID)

	// Prepare metadata for the agent
	metadata := make(map[string]string)
	metadata["correlation_id"] = correlationID
	if conversationID != "" {
		metadata["conversation_id"] = conversationID
	}

	// Call the Agent Runner gRPC service
	req := &pb.ProcessMessageRequest{
		UserId:    userID,
		Message:   message,
		SessionId: conversationID,
		Metadata:  metadata,
	}

	// Propagate userID via metadata
	ctx = middleware.PassUserIDToMetadata(ctx)

	rpcCtx, cancel := context.WithTimeout(ctx, agentRPCTimeout)
	defer cancel()
	resp, err := s.grpcClient.ProcessMessage(rpcCtx, req)
	if err != nil {
		s.logger.Error("Failed to call agent ProcessMessage", "error", err, "correlation_id", correlationID)
		return "", nil, fmt.Errorf("failed to process message with agent: %w", err)
	}

	if !resp.Success {
		errMsg := resp.Error
		if errMsg == "" {
			errMsg = "unknown error from agent"
		}
		s.logger.Error("Agent returned error", "error", errMsg, "correlation_id", correlationID)
		return "", nil, fmt.Errorf("agent error: %s", errMsg)
	}

	// Parse the response for proposed actions
	// The interceptor creates pending actions when the agent calls CalendarService CUD methods
	// We retrieve the most recent pending action for this conversation
	proposedAction := s.parseProposedAction(ctx, resp, conversationID, correlationID)

	return resp.Response, proposedAction, nil
}

func (s *Service) executeAction(ctx context.Context, action *models.PendingAction) (map[string]interface{}, string, error) {
	s.logger.Info("Executing action", "action_id", action.ActionID, "action_type", action.ActionType)

	// Parse proposed action
	var actionData map[string]interface{}
	if err := json.Unmarshal(action.ProposedAction, &actionData); err != nil {
		return nil, "", fmt.Errorf("failed to parse action data: %w", err)
	}

	// Execute based on action type
	switch action.ActionType {
	case "create_event":
		return s.executeCreateEvent(ctx, actionData)
	case "update_event":
		return s.executeUpdateEvent(ctx, actionData)
	case "delete_event":
		return s.executeDeleteEvent(ctx, actionData)
	default:
		return nil, "", fmt.Errorf("unsupported action type: %s", action.ActionType)
	}
}

func (s *Service) executeCreateEvent(ctx context.Context, actionData map[string]interface{}) (map[string]interface{}, string, error) {
	client := s.grpcClient.GetCalendarServiceClient()

	// Propagate userID via metadata
	ctx = middleware.PassUserIDToMetadata(ctx)

	// Extract event data
	title, _ := actionData["title"].(string)
	description, _ := actionData["description"].(string)
	location, _ := actionData["location"].(string)
	startTime, endTime, _, _ := extractTimeFields(actionData)

	req := &pb.CreateEventRequest{
		Title:       title,
		Description: description,
		StartTime:   startTime,
		EndTime:     endTime,
		Location:    location,
	}

	rpcCtx, cancel := context.WithTimeout(ctx, actionRPCTimeout)
	defer cancel()
	res, err := client.CreateEvent(rpcCtx, req)
	if err != nil {
		return nil, "", fmt.Errorf("gRPC CreateEvent failed: %w", err)
	}

	if !res.Success {
		return nil, "", fmt.Errorf("create event failed: %s", res.Message)
	}

	result := map[string]interface{}{
		"event_id": res.Event.Id,
		"message":  res.Message,
	}

	return result, res.Event.Id, nil
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

func (s *Service) executeUpdateEvent(ctx context.Context, actionData map[string]interface{}) (map[string]interface{}, string, error) {
	client := s.grpcClient.GetCalendarServiceClient()

	// Propagate userID via metadata
	ctx = middleware.PassUserIDToMetadata(ctx)

	// Extract event data
	eventID, _ := actionData["id"].(string)
	title, _ := actionData["title"].(string)
	description, _ := actionData["description"].(string)
	location, _ := actionData["location"].(string)
	startTime, endTime, _, _ := extractTimeFields(actionData)

	req := &pb.UpdateEventRequest{
		Id:          eventID,
		Title:       title,
		Description: description,
		StartTime:   startTime,
		EndTime:     endTime,
		Location:    location,
	}

	rpcCtx, cancel := context.WithTimeout(ctx, actionRPCTimeout)
	defer cancel()
	res, err := client.UpdateEvent(rpcCtx, req)
	if err != nil {
		return nil, "", fmt.Errorf("gRPC UpdateEvent failed: %w", err)
	}

	if !res.Success {
		return nil, "", fmt.Errorf("update event failed: %s", res.Message)
	}

	result := map[string]interface{}{
		"event_id": res.Event.Id,
		"message":  res.Message,
	}

	return result, res.Event.Id, nil
}

func (s *Service) executeDeleteEvent(ctx context.Context, actionData map[string]interface{}) (map[string]interface{}, string, error) {
	client := s.grpcClient.GetCalendarServiceClient()

	// Propagate userID via metadata
	ctx = middleware.PassUserIDToMetadata(ctx)

	// Extract event ID
	eventID, _ := actionData["id"].(string)

	req := &pb.DeleteEventRequest{
		Id: eventID,
	}

	rpcCtx, cancel := context.WithTimeout(ctx, actionRPCTimeout)
	defer cancel()
	res, err := client.DeleteEvent(rpcCtx, req)
	if err != nil {
		return nil, "", fmt.Errorf("gRPC DeleteEvent failed: %w", err)
	}

	if !res.Success {
		return nil, "", fmt.Errorf("delete event failed: %s", res.Message)
	}

	result := map[string]interface{}{
		"message": res.Message,
	}

	return result, eventID, nil
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
	// This is a simplified conflict check
	// In production, this would:
	// 1. Check for overlapping calendar events
	// 2. Check for concurrent modifications
	// 3. Validate resource availability
	// 4. Check user permissions

	if action.ActionType == "create_event" || action.ActionType == "update_event" {
		var eventData map[string]interface{}
		if err := json.Unmarshal(action.ProposedAction, &eventData); err != nil {
			return fmt.Errorf("failed to parse event data: %w", err)
		}

		// Example: Check if the time slot would conflict with existing events
		// This is a placeholder - actual implementation would query the calendar service
		startTime, endTime, hasStart, hasEnd := extractTimeFields(eventData)

		if hasStart && hasEnd {
			// In production, query calendar events in this time range
			// and check for overlaps
			s.logger.Info("Conflict check",
				"action_id", action.ActionID,
				"start_time", time.Unix(startTime, 0),
				"end_time", time.Unix(endTime, 0),
			)
		}
	}

	return nil
}

// parseProposedAction retrieves pending actions created by the gRPC interceptor
// When the agent calls CalendarService methods, the interceptor creates PendingAction records
// We retrieve the most recent one for this conversation and return it
func (s *Service) parseProposedAction(ctx context.Context, _ *pb.ProcessMessageResponse, conversationID, correlationID string) *ProposedAction {
	// Get pending actions created during this conversation/correlation
	// The interceptor stores them with the correlation_id
	pendingActions, err := s.repo.GetPendingActionsByConversation(ctx, conversationID)
	if err != nil {
		s.logger.Error("Failed to get pending actions", "error", err, "conversation_id", conversationID)
		return nil
	}

	// Find the most recently created pending action with matching correlation ID or status=pending
	var latestAction *models.PendingAction
	for _, action := range pendingActions {
		if action.Status != "pending" {
			continue
		}

		// Match by correlation ID if available, or just take the most recent pending action
		if action.CorrelationID == correlationID || latestAction == nil {
			if latestAction == nil || action.CreatedAt.After(latestAction.CreatedAt) {
				latestAction = action
			}
		}
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

// GET /chat/metrics - Get chat metrics
func (s *Service) handleGetMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	m := metrics.GetInstance()
	snapshot := m.GetSnapshot()

	if err := json.NewEncoder(w).Encode(snapshot); err != nil {
		s.logger.Error("Failed to encode response", "error", err)
	}
}
