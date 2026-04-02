package chat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/middleware"
	pb "github.com/waydxd/Orbit-core/proto/calendar"
)

// forwardToAgent sends the user message to the remote orbi agent via gRPC and
// returns the agent reply plus any proposed action captured by the interceptor.
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

// executeAction dispatches a confirmed pending action to the appropriate gRPC call.
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
		return s.executeCreateEvent(ctx, actionData, action.UserID)
	case "update_event":
		return s.executeUpdateEvent(ctx, actionData, action.UserID)
	case "delete_event":
		return s.executeDeleteEvent(ctx, actionData, action.UserID)
	default:
		return nil, "", fmt.Errorf("unsupported action type: %s", action.ActionType)
	}
}

func (s *Service) executeCreateEvent(ctx context.Context, actionData map[string]interface{}, userID string) (map[string]interface{}, string, error) {
	// Propagate userID via metadata
	ctx = middleware.PassUserIDToMetadata(ctx)

	// Extract event data
	title, _ := actionData["title"].(string)
	description, _ := actionData["description"].(string)
	location, _ := actionData["location"].(string)
	startTime, endTime, _, _ := extractTimeFields(actionData)

	req := &pb.CreateEventRequest{
		UserId:      userID,
		Title:       title,
		Description: description,
		StartTime:   startTime,
		EndTime:     endTime,
		Location:    location,
	}

	rpcCtx, cancel := context.WithTimeout(ctx, actionRPCTimeout)
	defer cancel()
	res, err := s.calendarService.CreateEvent(rpcCtx, req)
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

func (s *Service) executeUpdateEvent(ctx context.Context, actionData map[string]interface{}, userID string) (map[string]interface{}, string, error) {
	// Propagate userID via metadata
	ctx = middleware.PassUserIDToMetadata(ctx)

	// Extract event data
	eventID, _ := actionData["id"].(string)
	title, _ := actionData["title"].(string)
	description, _ := actionData["description"].(string)
	location, _ := actionData["location"].(string)
	startTime, endTime, _, _ := extractTimeFields(actionData)

	req := &pb.UpdateEventRequest{
		UserId:      userID,
		Id:          eventID,
		Title:       title,
		Description: description,
		StartTime:   startTime,
		EndTime:     endTime,
		Location:    location,
	}

	rpcCtx, cancel := context.WithTimeout(ctx, actionRPCTimeout)
	defer cancel()
	res, err := s.calendarService.UpdateEvent(rpcCtx, req)
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

func (s *Service) executeDeleteEvent(ctx context.Context, actionData map[string]interface{}, userID string) (map[string]interface{}, string, error) {
	// Propagate userID via metadata
	ctx = middleware.PassUserIDToMetadata(ctx)

	// Extract event ID
	eventID, _ := actionData["id"].(string)

	req := &pb.DeleteEventRequest{
		UserId: userID,
		Id:     eventID,
	}

	rpcCtx, cancel := context.WithTimeout(ctx, actionRPCTimeout)
	defer cancel()
	res, err := s.calendarService.DeleteEvent(rpcCtx, req)
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
