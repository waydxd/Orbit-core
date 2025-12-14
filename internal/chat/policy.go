package chat

import (
	"encoding/json"
	"fmt"
	"time"
)

// PolicyValidator validates proposed actions against business rules
type PolicyValidator struct {
	maxAttendeesPerEvent int
	maxEventsPerDay      int
	minEventDuration     time.Duration
	maxEventDuration     time.Duration
}

// NewPolicyValidator creates a new policy validator with default rules
func NewPolicyValidator() *PolicyValidator {
	return &PolicyValidator{
		maxAttendeesPerEvent: 50,
		maxEventsPerDay:      20,
		minEventDuration:     15 * time.Minute,
		maxEventDuration:     8 * time.Hour,
	}
}

// ValidationError represents a policy validation error
type ValidationError struct {
	Field   string
	Message string
	Code    string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateAction validates a proposed action against business rules
func (pv *PolicyValidator) ValidateAction(actionType string, actionData json.RawMessage) error {
	var data map[string]interface{}
	if err := json.Unmarshal(actionData, &data); err != nil {
		return &ValidationError{
			Field:   "proposed_action",
			Message: "Invalid action data format",
			Code:    "invalid_format",
		}
	}

	switch actionType {
	case "create_event":
		return pv.validateCreateEvent(data)
	case "update_event":
		return pv.validateUpdateEvent(data)
	case "delete_event":
		return pv.validateDeleteEvent(data)
	default:
		return &ValidationError{
			Field:   "action_type",
			Message: fmt.Sprintf("Unsupported action type: %s", actionType),
			Code:    "unsupported_action_type",
		}
	}
}

func (pv *PolicyValidator) validateCreateEvent(data map[string]interface{}) error {
	// Validate required fields
	title, ok := data["title"].(string)
	if !ok || title == "" {
		return &ValidationError{
			Field:   "title",
			Message: "Event title is required",
			Code:    "missing_title",
		}
	}

	// Validate time fields
	startTime, ok := data["start_time"].(float64)
	if !ok || startTime == 0 {
		return &ValidationError{
			Field:   "start_time",
			Message: "Valid start time is required",
			Code:    "invalid_start_time",
		}
	}

	endTime, ok := data["end_time"].(float64)
	if !ok || endTime == 0 {
		return &ValidationError{
			Field:   "end_time",
			Message: "Valid end time is required",
			Code:    "invalid_end_time",
		}
	}

	// Validate time range
	start := time.Unix(int64(startTime), 0)
	end := time.Unix(int64(endTime), 0)

	if end.Before(start) || end.Equal(start) {
		return &ValidationError{
			Field:   "end_time",
			Message: "End time must be after start time",
			Code:    "invalid_time_range",
		}
	}

	duration := end.Sub(start)
	if duration < pv.minEventDuration {
		return &ValidationError{
			Field:   "duration",
			Message: fmt.Sprintf("Event duration must be at least %v", pv.minEventDuration),
			Code:    "duration_too_short",
		}
	}

	if duration > pv.maxEventDuration {
		return &ValidationError{
			Field:   "duration",
			Message: fmt.Sprintf("Event duration cannot exceed %v", pv.maxEventDuration),
			Code:    "duration_too_long",
		}
	}

	// Validate past events
	if start.Before(time.Now().Add(-1 * time.Hour)) {
		return &ValidationError{
			Field:   "start_time",
			Message: "Cannot create events more than 1 hour in the past",
			Code:    "past_event",
		}
	}

	// Validate attendees if present
	if attendees, ok := data["attendees"].([]interface{}); ok {
		if len(attendees) > pv.maxAttendeesPerEvent {
			return &ValidationError{
				Field:   "attendees",
				Message: fmt.Sprintf("Cannot have more than %d attendees", pv.maxAttendeesPerEvent),
				Code:    "too_many_attendees",
			}
		}
	}

	return nil
}

func (pv *PolicyValidator) validateUpdateEvent(data map[string]interface{}) error {
	// Validate event ID
	eventID, ok := data["id"].(string)
	if !ok || eventID == "" {
		return &ValidationError{
			Field:   "id",
			Message: "Event ID is required for update",
			Code:    "missing_event_id",
		}
	}

	// If time fields are present, validate them
	if startTime, ok := data["start_time"].(float64); ok {
		if endTime, ok2 := data["end_time"].(float64); ok2 {
			start := time.Unix(int64(startTime), 0)
			end := time.Unix(int64(endTime), 0)

			if end.Before(start) || end.Equal(start) {
				return &ValidationError{
					Field:   "end_time",
					Message: "End time must be after start time",
					Code:    "invalid_time_range",
				}
			}

			duration := end.Sub(start)
			if duration < pv.minEventDuration {
				return &ValidationError{
					Field:   "duration",
					Message: fmt.Sprintf("Event duration must be at least %v", pv.minEventDuration),
					Code:    "duration_too_short",
				}
			}

			if duration > pv.maxEventDuration {
				return &ValidationError{
					Field:   "duration",
					Message: fmt.Sprintf("Event duration cannot exceed %v", pv.maxEventDuration),
					Code:    "duration_too_long",
				}
			}
		}
	}

	return nil
}

func (pv *PolicyValidator) validateDeleteEvent(data map[string]interface{}) error {
	// Validate event ID
	eventID, ok := data["id"].(string)
	if !ok || eventID == "" {
		return &ValidationError{
			Field:   "id",
			Message: "Event ID is required for deletion",
			Code:    "missing_event_id",
		}
	}

	return nil
}

// ValidateBulkAction validates that the action is not a mass delete/update
func (pv *PolicyValidator) ValidateBulkAction(actionType string, actionData json.RawMessage) error {
	// Prevent bulk deletes without explicit IDs
	if actionType == "delete_event" {
		var data map[string]interface{}
		if err := json.Unmarshal(actionData, &data); err != nil {
			return err
		}

		// Check if this is attempting a bulk delete pattern
		if _, hasFilter := data["filter"]; hasFilter {
			return &ValidationError{
				Field:   "action_type",
				Message: "Bulk delete operations are not allowed",
				Code:    "bulk_delete_forbidden",
			}
		}
	}

	return nil
}
