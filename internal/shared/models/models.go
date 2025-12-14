package models

import (
	"encoding/json"
	"time"
)

// User represents a user in the system
type User struct {
	ID           string    `json:"id" db:"id"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	FirstName    string    `json:"first_name" db:"first_name"`
	LastName     string    `json:"last_name" db:"last_name"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// Event represents a calendar event
type Event struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	Title       string    `json:"title" db:"title"`
	Description string    `json:"description" db:"description"`
	StartTime   time.Time `json:"start_time" db:"start_time"`
	EndTime     time.Time `json:"end_time" db:"end_time"`
	Location    string    `json:"location" db:"location"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
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
	ID            string    `json:"id" db:"id"`
	UserID        string    `json:"user_id" db:"user_id"`
	CorrelationID string    `json:"correlation_id" db:"correlation_id"`
	Status        string    `json:"status" db:"status"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// ChatMessage represents a message in a conversation
type ChatMessage struct {
	ID             string          `json:"id" db:"id"`
	ConversationID string          `json:"conversation_id" db:"conversation_id"`
	UserID         string          `json:"user_id" db:"user_id"`
	Role           string          `json:"role" db:"role"`
	Content        string          `json:"content" db:"content"`
	Metadata       json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
}

// PendingAction represents an action proposed by the agent awaiting user confirmation
type PendingAction struct {
	ID             string          `json:"id" db:"id"`
	ActionID       string          `json:"action_id" db:"action_id"`
	UserID         string          `json:"user_id" db:"user_id"`
	ConversationID string          `json:"conversation_id" db:"conversation_id"`
	ProposedAction json.RawMessage `json:"proposed_action" db:"proposed_action"`
	ActionType     string          `json:"action_type" db:"action_type"`
	IdempotencyKey string          `json:"idempotency_key" db:"idempotency_key"`
	Status         string          `json:"status" db:"status"`
	Version        int             `json:"version" db:"version"`
	CorrelationID  string          `json:"correlation_id" db:"correlation_id"`
	AgentMetadata  json.RawMessage `json:"agent_metadata,omitempty" db:"agent_metadata"`
	ErrorMessage   string          `json:"error_message,omitempty" db:"error_message"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at" db:"updated_at"`
	ExpiresAt      time.Time       `json:"expires_at" db:"expires_at"`
}

// AgentToolLog represents an audit log of agent tool calls
type AgentToolLog struct {
	ID              string          `json:"id" db:"id"`
	PendingActionID *string         `json:"pending_action_id,omitempty" db:"pending_action_id"`
	ConversationID  string          `json:"conversation_id" db:"conversation_id"`
	UserID          string          `json:"user_id" db:"user_id"`
	ToolName        string          `json:"tool_name" db:"tool_name"`
	ToolInput       json.RawMessage `json:"tool_input" db:"tool_input"`
	ToolOutput      json.RawMessage `json:"tool_output,omitempty" db:"tool_output"`
	Status          string          `json:"status" db:"status"`
	ErrorMessage    string          `json:"error_message,omitempty" db:"error_message"`
	CorrelationID   string          `json:"correlation_id" db:"correlation_id"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
}
