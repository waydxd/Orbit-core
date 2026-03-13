package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/metrics"
	"github.com/waydxd/Orbit-core/pkg/middleware"
)

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
	}
	if messages == nil {
		messages = []*models.ChatMessage{}
	}

	// Get pending actions
	pendingActions, err := s.repo.GetPendingActionsByConversation(ctx, conversationID)
	if err != nil {
		s.logger.Error("Failed to get pending actions", "error", err)
	}
	if pendingActions == nil {
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

// GET /chat/metrics - Get chat metrics
func (s *Service) handleGetMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	m := metrics.GetInstance()
	snapshot := m.GetSnapshot()

	if err := json.NewEncoder(w).Encode(snapshot); err != nil {
		s.logger.Error("Failed to encode response", "error", err)
	}
}
