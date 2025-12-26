package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/logger"
	pb "github.com/waydxd/Orbit-core/proto/calendar"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// PendingActionStore defines the interface for storing pending actions
type PendingActionStore interface {
	CreatePendingAction(ctx context.Context, action *models.PendingAction) (*models.PendingAction, error)
}

// ActionInterceptor intercepts mutating Calendar operations and converts them to pending actions
type ActionInterceptor struct {
	logger      *logger.Logger
	actionStore PendingActionStore
}

// NewActionInterceptor creates a new action interceptor
func NewActionInterceptor(log *logger.Logger, store PendingActionStore) *ActionInterceptor {
	return &ActionInterceptor{
		logger:      log,
		actionStore: store,
	}
}

// UnaryInterceptor returns a gRPC unary server interceptor that captures mutating operations
func (i *ActionInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Check if this is a mutating CalendarService operation
		if shouldIntercept(info.FullMethod) {
			return i.interceptMutatingOperation(ctx, req, info, handler)
		}

		// For non-mutating operations, pass through
		return handler(ctx, req)
	}
}

// shouldIntercept determines if a method should be intercepted for confirmation
func shouldIntercept(method string) bool {
	mutatingMethods := map[string]bool{
		"/calendar.CalendarService/CreateEvent": true,
		"/calendar.CalendarService/UpdateEvent": true,
		"/calendar.CalendarService/DeleteEvent": true,
	}
	return mutatingMethods[method]
}

// interceptMutatingOperation captures the operation and stores it as a pending action
func (i *ActionInterceptor) interceptMutatingOperation(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	i.logger.Info("Intercepting mutating operation", "method", info.FullMethod)

	// handler is intentionally unused because mutating operations are converted into pending actions
	_ = handler

	// Extract metadata from context (user_id, session_id, etc.)
	userID, sessionID := extractMetadata(ctx)

	// If userID is not present in metadata, some CalendarService requests include it in the request payload
	if userID == "" {
		switch r := req.(type) {
		case *pb.CreateEventRequest:
			userID = r.UserId
		case *pb.UpdateEventRequest:
			userID = r.UserId
		case *pb.DeleteEventRequest:
			userID = r.UserId
		default:
			// leave userID empty if we don't recognize the request type
		}
	}

	// Convert the request to a pending action
	actionType, actionData, summary, err := convertRequestToAction(info.FullMethod, req)
	if err != nil {
		i.logger.Error("Failed to convert request to action", "error", err)
		return nil, fmt.Errorf("failed to convert request to pending action: %w", err)
	}

	// Serialize action data
	actionJSON, err := json.Marshal(actionData)
	if err != nil {
		i.logger.Error("Failed to marshal action data", "error", err)
		return nil, fmt.Errorf("failed to serialize action: %w", err)
	}

	// Create pending action
	actionID := generateActionID()
	idempotencyKey := generateIdempotencyKey(userID, sessionID, actionType)

	pendingAction := &models.PendingAction{
		ActionID:       actionID,
		UserID:         userID,
		ConversationID: sessionID,
		ProposedAction: actionJSON,
		ActionType:     actionType,
		IdempotencyKey: idempotencyKey,
		Status:         "pending",
		CorrelationID:  extractCorrelationID(ctx),
		ExpiresAt:      time.Now().Add(24 * time.Hour),
	}

	// Store the pending action
	_, err = i.actionStore.CreatePendingAction(ctx, pendingAction)
	if err != nil {
		i.logger.Error("Failed to store pending action", "error", err)
		return nil, fmt.Errorf("failed to store pending action: %w", err)
	}

	i.logger.Info("Created pending action", "action_id", actionID, "action_type", actionType)

	// Return a response indicating action is pending confirmation
	return createPendingResponse(info.FullMethod, actionID, summary)
}

// extractMetadata extracts user_id and session_id from the request context
func extractMetadata(ctx context.Context) (userID, sessionID string) {
	// Try to read from gRPC incoming metadata (common keys and variants)
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		// helper to try multiple key variants
		getFirst := func(keys ...string) string {
			for _, k := range keys {
				if vals := md.Get(k); len(vals) > 0 {
					return vals[0]
				}
			}
			return ""
		}

		// user id may be provided under several possible keys
		userID = getFirst("user_id", "user-id", "userid", "user")
		// session or conversation id may be provided under these keys
		sessionID = getFirst("conversation_id", "conversation-id", "session_id", "session-id", "session", "conversation")
		return userID, sessionID
	}

	// No metadata present; return empty values
	return "", ""
}

// extractCorrelationID extracts correlation ID from context metadata
func extractCorrelationID(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if correlationIDs := md.Get("correlation_id"); len(correlationIDs) > 0 {
			return correlationIDs[0]
		}
	}
	// Generate a new one if not found
	return fmt.Sprintf("corr_%d", time.Now().UnixNano())
}

// generateActionID generates a unique action ID
func generateActionID() string {
	return fmt.Sprintf("action_%d", time.Now().UnixNano())
}

// generateIdempotencyKey generates an idempotency key
func generateIdempotencyKey(userID, sessionID, actionType string) string {
	return fmt.Sprintf("%s_%s_%s_%d", userID, sessionID, actionType, time.Now().UnixNano())
}

// convertRequestToAction converts a gRPC request to action data
func convertRequestToAction(method string, req interface{}) (actionType string, actionData map[string]interface{}, summary string, err error) {
	switch method {
	case "/calendar.CalendarService/CreateEvent":
		return convertCreateEventRequest(req.(*pb.CreateEventRequest))
	case "/calendar.CalendarService/UpdateEvent":
		return convertUpdateEventRequest(req.(*pb.UpdateEventRequest))
	case "/calendar.CalendarService/DeleteEvent":
		return convertDeleteEventRequest(req.(*pb.DeleteEventRequest))
	default:
		return "", nil, "", fmt.Errorf("unsupported method: %s", method)
	}
}

func convertCreateEventRequest(req *pb.CreateEventRequest) (string, map[string]interface{}, string, error) {
	actionData := map[string]interface{}{
		"user_id":     req.UserId,
		"title":       req.Title,
		"description": req.Description,
		"start_time":  req.StartTime,
		"end_time":    req.EndTime,
		"location":    req.Location,
		"attendees":   req.Attendees,
		"recurrence":  req.Recurrence,
		"status":      req.Status,
	}

	summary := fmt.Sprintf("Create event: %s", req.Title)
	if req.Location != "" {
		summary += fmt.Sprintf(" at %s", req.Location)
	}

	return "create_event", actionData, summary, nil
}

func convertUpdateEventRequest(req *pb.UpdateEventRequest) (string, map[string]interface{}, string, error) {
	actionData := map[string]interface{}{
		"id":          req.Id,
		"user_id":     req.UserId,
		"title":       req.Title,
		"description": req.Description,
		"start_time":  req.StartTime,
		"end_time":    req.EndTime,
		"location":    req.Location,
		"attendees":   req.Attendees,
		"recurrence":  req.Recurrence,
		"status":      req.Status,
	}

	summary := fmt.Sprintf("Update event: %s", req.Title)

	return "update_event", actionData, summary, nil
}

func convertDeleteEventRequest(req *pb.DeleteEventRequest) (string, map[string]interface{}, string, error) {
	actionData := map[string]interface{}{
		"id":      req.Id,
		"user_id": req.UserId,
	}

	summary := fmt.Sprintf("Delete event: %s", req.Id)

	return "delete_event", actionData, summary, nil
}

// createPendingResponse creates a response indicating the action is pending
func createPendingResponse(method string, actionID, summary string) (interface{}, error) {
	switch method {
	case "/calendar.CalendarService/CreateEvent":
		return &pb.CreateEventResponse{
			Success: false,
			Message: fmt.Sprintf("Action pending confirmation. Action ID: %s. %s", actionID, summary),
		}, nil
	case "/calendar.CalendarService/UpdateEvent":
		return &pb.UpdateEventResponse{
			Success: false,
			Message: fmt.Sprintf("Action pending confirmation. Action ID: %s. %s", actionID, summary),
		}, nil
	case "/calendar.CalendarService/DeleteEvent":
		return &pb.DeleteEventResponse{
			Success: false,
			Message: fmt.Sprintf("Action pending confirmation. Action ID: %s. %s", actionID, summary),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
	}
}
