package models

import (
	"encoding/json"
	"time"
)

// User represents a user in the system
type User struct {
	ID             string    `json:"id" db:"id"`
	Email          string    `json:"email" db:"email"`
	PasswordHash   string    `json:"-" db:"password_hash"`
	FirstName      string    `json:"first_name" db:"first_name"`
	LastName       string    `json:"last_name" db:"last_name"`
	EmailVerified  bool      `json:"email_verified" db:"email_verified"`
	Username       string    `json:"username" db:"username"`
	ProfilePicture string    `json:"profile_picture,omitempty" db:"profile_picture"`
	Region         string    `json:"region,omitempty" db:"region"`
	Timezone       string    `json:"timezone,omitempty" db:"timezone"`
	Gender         string    `json:"gender,omitempty" db:"gender"`
	BirthDate      time.Time `json:"birth_date,omitempty" db:"birth_date"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// Event represents a calendar event
type Event struct {
	ID                  string    `json:"id" db:"id"`
	UserID              string    `json:"user_id" db:"user_id"`
	Title               string    `json:"title" db:"title"`
	Description         string    `json:"description" db:"description"`
	StartTime           time.Time `json:"start_time" db:"start_time"`
	EndTime             time.Time `json:"end_time" db:"end_time"`
	Location            string    `json:"location" db:"location"`
	Hashtag             []string  `json:"hashtag" db:"hashtag"`
	IsRecurring         bool      `json:"is_recurring" db:"is_recurring"`
	RecurrenceRule      string    `json:"recurrence_rule" db:"recurrence_rule"`
	RecurrenceException string    `json:"recurrence_exception" db:"recurrence_exception"`
	CreatedAt           time.Time `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time `json:"updated_at" db:"updated_at"`
}

// Task represents a task item
type Task struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	Title       string    `json:"title" db:"title"`
	Description string    `json:"description" db:"description"`
	DueDate     time.Time `json:"due_date" db:"due_date"`
	Completed   bool      `json:"completed" db:"completed"`
	Priority    string    `json:"priority" db:"priority"`
	Hashtag     []string  `json:"hashtag" db:"hashtag"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// Location represents a location record
type Location struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Latitude  float64   `json:"latitude" db:"latitude"`
	Longitude float64   `json:"longitude" db:"longitude"`
	Address   string    `json:"address" db:"address"`
	Timestamp time.Time `json:"timestamp" db:"timestamp"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// Session represents an authentication session
type Session struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	TokenHash string    `json:"-" db:"token_hash"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// Integration represents an external service integration
type Integration struct {
	ID              string    `json:"id" db:"id"`
	UserID          string    `json:"user_id" db:"user_id"`
	ServiceName     string    `json:"service_name" db:"service_name"`
	APIKeyEncrypted string    `json:"-" db:"api_key_encrypted"`
	Status          string    `json:"status" db:"status"`
	LastSync        time.Time `json:"last_sync" db:"last_sync"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// Conversation represents a chat conversation
type Conversation struct {
	ID            string    `json:"id" db:"id" bson:"id"`
	UserID        string    `json:"user_id" db:"user_id" bson:"user_id"`
	CorrelationID string    `json:"correlation_id" db:"correlation_id" bson:"correlation_id"`
	Status        string    `json:"status" db:"status" bson:"status"`
	CreatedAt     time.Time `json:"created_at" db:"created_at" bson:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at" bson:"updated_at"`
}

// ChatMessage represents a message in a conversation
type ChatMessage struct {
	ID             string          `json:"id" db:"id" bson:"id"`
	ConversationID string          `json:"conversation_id" db:"conversation_id" bson:"conversation_id"`
	UserID         string          `json:"user_id" db:"user_id" bson:"user_id"`
	Role           string          `json:"role" db:"role" bson:"role"`
	Content        string          `json:"content" db:"content" bson:"content"`
	Metadata       json.RawMessage `json:"metadata,omitempty" db:"metadata" bson:"metadata,omitempty"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at" bson:"created_at"`
}

// PendingAction represents an action proposed by the agent awaiting user confirmation
type PendingAction struct {
	ID             string          `json:"id" db:"id" bson:"id"`
	ActionID       string          `json:"action_id" db:"action_id" bson:"action_id"`
	UserID         string          `json:"user_id" db:"user_id" bson:"user_id"`
	ConversationID string          `json:"conversation_id" db:"conversation_id" bson:"conversation_id"`
	ProposedAction json.RawMessage `json:"proposed_action" db:"proposed_action" bson:"proposed_action"`
	ActionType     string          `json:"action_type" db:"action_type" bson:"action_type"`
	IdempotencyKey string          `json:"idempotency_key" db:"idempotency_key" bson:"idempotency_key"`
	Status         string          `json:"status" db:"status" bson:"status"`
	Version        int             `json:"version" db:"version" bson:"version"`
	CorrelationID  string          `json:"correlation_id" db:"correlation_id" bson:"correlation_id"`
	AgentMetadata  json.RawMessage `json:"agent_metadata,omitempty" db:"agent_metadata" bson:"agent_metadata,omitempty"`
	ErrorMessage   string          `json:"error_message,omitempty" db:"error_message" bson:"error_message,omitempty"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at" bson:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at" db:"updated_at" bson:"updated_at"`
	ExpiresAt      time.Time       `json:"expires_at" db:"expires_at" bson:"expires_at"`
}

// AgentToolLog represents an audit log of agent tool calls
type AgentToolLog struct {
	ID              string          `json:"id" db:"id" bson:"id"`
	PendingActionID *string         `json:"pending_action_id,omitempty" db:"pending_action_id" bson:"pending_action_id,omitempty"`
	ConversationID  string          `json:"conversation_id" db:"conversation_id" bson:"conversation_id"`
	UserID          string          `json:"user_id" db:"user_id" bson:"user_id"`
	ToolName        string          `json:"tool_name" db:"tool_name" bson:"tool_name"`
	ToolInput       json.RawMessage `json:"tool_input" db:"tool_input" bson:"tool_input"`
	ToolOutput      json.RawMessage `json:"tool_output,omitempty" db:"tool_output" bson:"tool_output,omitempty"`
	Status          string          `json:"status" db:"status" bson:"status"`
	ErrorMessage    string          `json:"error_message,omitempty" db:"error_message" bson:"error_message,omitempty"`
	CorrelationID   string          `json:"correlation_id" db:"correlation_id" bson:"correlation_id"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at" bson:"created_at"`
}

// EventFrequency tracks the frequency of similar events for habit detection
type EventFrequency struct {
	ID                   string      `json:"id" db:"id"`
	UserID               string      `json:"user_id" db:"user_id"`
	Title                string      `json:"title" db:"title"`
	Description          string      `json:"description" db:"description"`
	Location             string      `json:"location" db:"location"`
	DurationMinutes      int         `json:"duration_minutes" db:"duration_minutes"`
	TimeOfDay            int         `json:"time_of_day" db:"time_of_day"` // Minutes from midnight
	DayOfWeek            int         `json:"day_of_week" db:"day_of_week"` // 0-6, Sunday = 0
	OccurrenceCount      int         `json:"occurrence_count" db:"occurrence_count"`
	SuggestionThreshold  int         `json:"suggestion_threshold" db:"suggestion_threshold"`
	SuggestionShown      bool        `json:"suggestion_shown" db:"suggestion_shown"`
	HabitAccepted        bool        `json:"habit_accepted" db:"habit_accepted"`
	OccurrenceTimestamps []time.Time `json:"occurrence_timestamps" db:"occurrence_timestamps"`
	CreatedAt            time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time   `json:"updated_at" db:"updated_at"`
}

// HabitSuggestion represents a pending suggestion to create a recurring event
type HabitSuggestion struct {
	ID                string     `json:"id" db:"id"`
	UserID            string     `json:"user_id" db:"user_id"`
	EventFrequencyID  string     `json:"event_frequency_id" db:"event_frequency_id"`
	Title             string     `json:"title" db:"title"`
	Description       string     `json:"description" db:"description"`
	Location          string     `json:"location" db:"location"`
	DurationMinutes   int        `json:"duration_minutes" db:"duration_minutes"`
	TimeOfDay         int        `json:"time_of_day" db:"time_of_day"`
	DayOfWeek         int        `json:"day_of_week" db:"day_of_week"`
	Status            string     `json:"status" db:"status"` // pending, accepted, rejected, expired
	RecurrenceEndDate *time.Time `json:"recurrence_end_date,omitempty" db:"recurrence_end_date"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
	ExpiresAt         time.Time  `json:"expires_at" db:"expires_at"`
}
