package chat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/waydxd/Orbit-core/internal/shared/database"
	"github.com/waydxd/Orbit-core/internal/shared/models"
)

// Repository defines the interface for chat data operations
type Repository interface {
	// Conversation operations
	CreateConversation(ctx context.Context, userID string, correlationID string) (*models.Conversation, error)
	GetConversationByID(ctx context.Context, conversationID string) (*models.Conversation, error)
	GetConversationByCorrelationID(ctx context.Context, correlationID string) (*models.Conversation, error)
	ListConversationsByUser(ctx context.Context, userID string, limit int) ([]*models.Conversation, error)
	UpdateConversationStatus(ctx context.Context, conversationID string, status string) error

	// Message operations
	CreateMessage(ctx context.Context, msg *models.ChatMessage) (*models.ChatMessage, error)
	GetMessagesByConversation(ctx context.Context, conversationID string) ([]*models.ChatMessage, error)

	// Pending action operations
	CreatePendingAction(ctx context.Context, action *models.PendingAction) (*models.PendingAction, error)
	GetPendingActionByID(ctx context.Context, actionID string) (*models.PendingAction, error)
	GetPendingActionsByConversation(ctx context.Context, conversationID string) ([]*models.PendingAction, error)
	UpdatePendingActionStatus(ctx context.Context, actionID string, status string, version int, errorMsg string) error
	GetExpiredActions(ctx context.Context) ([]*models.PendingAction, error)

	// Tool log operations
	CreateToolLog(ctx context.Context, log *models.AgentToolLog) (*models.AgentToolLog, error)
	GetToolLogsByConversation(ctx context.Context, conversationID string) ([]*models.AgentToolLog, error)
	GetToolLogsByPendingAction(ctx context.Context, pendingActionID string) ([]*models.AgentToolLog, error)
}

// SQLRepository implements Repository using PostgreSQL
type SQLRepository struct {
	db *database.DB
}

// NewSQLRepository creates a new SQL repository
func NewSQLRepository(db *database.DB) Repository {
	return &SQLRepository{db: db}
}

// CreateConversation creates a new conversation
func (r *SQLRepository) CreateConversation(ctx context.Context, userID string, correlationID string) (*models.Conversation, error) {
	query := `
		INSERT INTO conversations (user_id, correlation_id, status)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, correlation_id, status, created_at, updated_at
	`
	
	var conv models.Conversation
	err := r.db.QueryRowContext(ctx, query, userID, correlationID, "active").Scan(
		&conv.ID, &conv.UserID, &conv.CorrelationID, &conv.Status, &conv.CreatedAt, &conv.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}
	
	return &conv, nil
}

// GetConversationByID retrieves a conversation by ID
func (r *SQLRepository) GetConversationByID(ctx context.Context, conversationID string) (*models.Conversation, error) {
	query := `
		SELECT id, user_id, correlation_id, status, created_at, updated_at
		FROM conversations
		WHERE id = $1
	`
	
	var conv models.Conversation
	err := r.db.QueryRowContext(ctx, query, conversationID).Scan(
		&conv.ID, &conv.UserID, &conv.CorrelationID, &conv.Status, &conv.CreatedAt, &conv.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("conversation not found")
		}
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}
	
	return &conv, nil
}

// GetConversationByCorrelationID retrieves a conversation by correlation ID
func (r *SQLRepository) GetConversationByCorrelationID(ctx context.Context, correlationID string) (*models.Conversation, error) {
	query := `
		SELECT id, user_id, correlation_id, status, created_at, updated_at
		FROM conversations
		WHERE correlation_id = $1
	`
	
	var conv models.Conversation
	err := r.db.QueryRowContext(ctx, query, correlationID).Scan(
		&conv.ID, &conv.UserID, &conv.CorrelationID, &conv.Status, &conv.CreatedAt, &conv.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("conversation not found")
		}
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}
	
	return &conv, nil
}

// ListConversationsByUser retrieves conversations for a user
func (r *SQLRepository) ListConversationsByUser(ctx context.Context, userID string, limit int) ([]*models.Conversation, error) {
	query := `
		SELECT id, user_id, correlation_id, status, created_at, updated_at
		FROM conversations
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	
	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}
	defer rows.Close()
	
	var conversations []*models.Conversation
	for rows.Next() {
		var conv models.Conversation
		err := rows.Scan(&conv.ID, &conv.UserID, &conv.CorrelationID, &conv.Status, &conv.CreatedAt, &conv.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}
		conversations = append(conversations, &conv)
	}
	
	return conversations, nil
}

// UpdateConversationStatus updates the status of a conversation
func (r *SQLRepository) UpdateConversationStatus(ctx context.Context, conversationID string, status string) error {
	query := `UPDATE conversations SET status = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, status, conversationID)
	if err != nil {
		return fmt.Errorf("failed to update conversation status: %w", err)
	}
	return nil
}

// CreateMessage creates a new chat message
func (r *SQLRepository) CreateMessage(ctx context.Context, msg *models.ChatMessage) (*models.ChatMessage, error) {
	query := `
		INSERT INTO chat_messages (conversation_id, user_id, role, content, metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, conversation_id, user_id, role, content, metadata, created_at
	`
	
	var message models.ChatMessage
	err := r.db.QueryRowContext(ctx, query, msg.ConversationID, msg.UserID, msg.Role, msg.Content, msg.Metadata).Scan(
		&message.ID, &message.ConversationID, &message.UserID, &message.Role, &message.Content, &message.Metadata, &message.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}
	
	return &message, nil
}

// GetMessagesByConversation retrieves messages for a conversation
func (r *SQLRepository) GetMessagesByConversation(ctx context.Context, conversationID string) ([]*models.ChatMessage, error) {
	query := `
		SELECT id, conversation_id, user_id, role, content, metadata, created_at
		FROM chat_messages
		WHERE conversation_id = $1
		ORDER BY created_at ASC
	`
	
	rows, err := r.db.QueryContext(ctx, query, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	defer rows.Close()
	
	var messages []*models.ChatMessage
	for rows.Next() {
		var msg models.ChatMessage
		err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.UserID, &msg.Role, &msg.Content, &msg.Metadata, &msg.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, &msg)
	}
	
	return messages, nil
}

// CreatePendingAction creates a new pending action
func (r *SQLRepository) CreatePendingAction(ctx context.Context, action *models.PendingAction) (*models.PendingAction, error) {
	query := `
		INSERT INTO pending_actions (action_id, user_id, conversation_id, proposed_action, action_type, idempotency_key, status, correlation_id, agent_metadata, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, action_id, user_id, conversation_id, proposed_action, action_type, idempotency_key, status, version, correlation_id, agent_metadata, error_message, created_at, updated_at, expires_at
	`
	
	var pa models.PendingAction
	err := r.db.QueryRowContext(ctx, query,
		action.ActionID, action.UserID, action.ConversationID, action.ProposedAction, action.ActionType,
		action.IdempotencyKey, action.Status, action.CorrelationID, action.AgentMetadata, action.ExpiresAt,
	).Scan(
		&pa.ID, &pa.ActionID, &pa.UserID, &pa.ConversationID, &pa.ProposedAction, &pa.ActionType,
		&pa.IdempotencyKey, &pa.Status, &pa.Version, &pa.CorrelationID, &pa.AgentMetadata, &pa.ErrorMessage,
		&pa.CreatedAt, &pa.UpdatedAt, &pa.ExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create pending action: %w", err)
	}
	
	return &pa, nil
}

// GetPendingActionByID retrieves a pending action by action ID
func (r *SQLRepository) GetPendingActionByID(ctx context.Context, actionID string) (*models.PendingAction, error) {
	query := `
		SELECT id, action_id, user_id, conversation_id, proposed_action, action_type, idempotency_key, status, version, correlation_id, agent_metadata, error_message, created_at, updated_at, expires_at
		FROM pending_actions
		WHERE action_id = $1
	`
	
	var pa models.PendingAction
	err := r.db.QueryRowContext(ctx, query, actionID).Scan(
		&pa.ID, &pa.ActionID, &pa.UserID, &pa.ConversationID, &pa.ProposedAction, &pa.ActionType,
		&pa.IdempotencyKey, &pa.Status, &pa.Version, &pa.CorrelationID, &pa.AgentMetadata, &pa.ErrorMessage,
		&pa.CreatedAt, &pa.UpdatedAt, &pa.ExpiresAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pending action not found")
		}
		return nil, fmt.Errorf("failed to get pending action: %w", err)
	}
	
	return &pa, nil
}

// GetPendingActionsByConversation retrieves pending actions for a conversation
func (r *SQLRepository) GetPendingActionsByConversation(ctx context.Context, conversationID string) ([]*models.PendingAction, error) {
	query := `
		SELECT id, action_id, user_id, conversation_id, proposed_action, action_type, idempotency_key, status, version, correlation_id, agent_metadata, error_message, created_at, updated_at, expires_at
		FROM pending_actions
		WHERE conversation_id = $1
		ORDER BY created_at DESC
	`
	
	rows, err := r.db.QueryContext(ctx, query, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending actions: %w", err)
	}
	defer rows.Close()
	
	var actions []*models.PendingAction
	for rows.Next() {
		var pa models.PendingAction
		err := rows.Scan(
			&pa.ID, &pa.ActionID, &pa.UserID, &pa.ConversationID, &pa.ProposedAction, &pa.ActionType,
			&pa.IdempotencyKey, &pa.Status, &pa.Version, &pa.CorrelationID, &pa.AgentMetadata, &pa.ErrorMessage,
			&pa.CreatedAt, &pa.UpdatedAt, &pa.ExpiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pending action: %w", err)
		}
		actions = append(actions, &pa)
	}
	
	return actions, nil
}

// UpdatePendingActionStatus updates a pending action with optimistic locking
func (r *SQLRepository) UpdatePendingActionStatus(ctx context.Context, actionID string, status string, version int, errorMsg string) error {
	query := `
		UPDATE pending_actions 
		SET status = $1, version = version + 1, error_message = $2
		WHERE action_id = $3 AND version = $4
	`
	
	result, err := r.db.ExecContext(ctx, query, status, errorMsg, actionID, version)
	if err != nil {
		return fmt.Errorf("failed to update pending action: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("version mismatch or action not found: optimistic lock failed")
	}
	
	return nil
}

// GetExpiredActions retrieves expired pending actions
func (r *SQLRepository) GetExpiredActions(ctx context.Context) ([]*models.PendingAction, error) {
	query := `
		SELECT id, action_id, user_id, conversation_id, proposed_action, action_type, idempotency_key, status, version, correlation_id, agent_metadata, error_message, created_at, updated_at, expires_at
		FROM pending_actions
		WHERE status = 'pending' AND expires_at < $1
	`
	
	rows, err := r.db.QueryContext(ctx, query, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get expired actions: %w", err)
	}
	defer rows.Close()
	
	var actions []*models.PendingAction
	for rows.Next() {
		var pa models.PendingAction
		err := rows.Scan(
			&pa.ID, &pa.ActionID, &pa.UserID, &pa.ConversationID, &pa.ProposedAction, &pa.ActionType,
			&pa.IdempotencyKey, &pa.Status, &pa.Version, &pa.CorrelationID, &pa.AgentMetadata, &pa.ErrorMessage,
			&pa.CreatedAt, &pa.UpdatedAt, &pa.ExpiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan expired action: %w", err)
		}
		actions = append(actions, &pa)
	}
	
	return actions, nil
}

// CreateToolLog creates an agent tool log
func (r *SQLRepository) CreateToolLog(ctx context.Context, log *models.AgentToolLog) (*models.AgentToolLog, error) {
	query := `
		INSERT INTO agent_tool_logs (pending_action_id, conversation_id, user_id, tool_name, tool_input, tool_output, status, error_message, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, pending_action_id, conversation_id, user_id, tool_name, tool_input, tool_output, status, error_message, correlation_id, created_at
	`
	
	var tl models.AgentToolLog
	err := r.db.QueryRowContext(ctx, query,
		log.PendingActionID, log.ConversationID, log.UserID, log.ToolName, log.ToolInput,
		log.ToolOutput, log.Status, log.ErrorMessage, log.CorrelationID,
	).Scan(
		&tl.ID, &tl.PendingActionID, &tl.ConversationID, &tl.UserID, &tl.ToolName, &tl.ToolInput,
		&tl.ToolOutput, &tl.Status, &tl.ErrorMessage, &tl.CorrelationID, &tl.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool log: %w", err)
	}
	
	return &tl, nil
}

// GetToolLogsByConversation retrieves tool logs for a conversation
func (r *SQLRepository) GetToolLogsByConversation(ctx context.Context, conversationID string) ([]*models.AgentToolLog, error) {
	query := `
		SELECT id, pending_action_id, conversation_id, user_id, tool_name, tool_input, tool_output, status, error_message, correlation_id, created_at
		FROM agent_tool_logs
		WHERE conversation_id = $1
		ORDER BY created_at ASC
	`
	
	rows, err := r.db.QueryContext(ctx, query, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool logs: %w", err)
	}
	defer rows.Close()
	
	var logs []*models.AgentToolLog
	for rows.Next() {
		var tl models.AgentToolLog
		err := rows.Scan(
			&tl.ID, &tl.PendingActionID, &tl.ConversationID, &tl.UserID, &tl.ToolName, &tl.ToolInput,
			&tl.ToolOutput, &tl.Status, &tl.ErrorMessage, &tl.CorrelationID, &tl.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tool log: %w", err)
		}
		logs = append(logs, &tl)
	}
	
	return logs, nil
}

// GetToolLogsByPendingAction retrieves tool logs for a pending action
func (r *SQLRepository) GetToolLogsByPendingAction(ctx context.Context, pendingActionID string) ([]*models.AgentToolLog, error) {
	query := `
		SELECT id, pending_action_id, conversation_id, user_id, tool_name, tool_input, tool_output, status, error_message, correlation_id, created_at
		FROM agent_tool_logs
		WHERE pending_action_id = $1
		ORDER BY created_at ASC
	`
	
	rows, err := r.db.QueryContext(ctx, query, pendingActionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool logs: %w", err)
	}
	defer rows.Close()
	
	var logs []*models.AgentToolLog
	for rows.Next() {
		var tl models.AgentToolLog
		err := rows.Scan(
			&tl.ID, &tl.PendingActionID, &tl.ConversationID, &tl.UserID, &tl.ToolName, &tl.ToolInput,
			&tl.ToolOutput, &tl.Status, &tl.ErrorMessage, &tl.CorrelationID, &tl.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tool log: %w", err)
		}
		logs = append(logs, &tl)
	}
	
	return logs, nil
}

// Helper function to generate action ID
func GenerateActionID() string {
	return fmt.Sprintf("action_%s", uuid.New().String())
}

// Helper function to generate correlation ID
func GenerateCorrelationID() string {
	return uuid.New().String()
}

// Helper function to generate idempotency key
func GenerateIdempotencyKey(userID, conversationID, actionType string, timestamp int64) string {
	data := fmt.Sprintf("%s:%s:%s:%d", userID, conversationID, actionType, timestamp)
	hash := uuid.NewSHA1(uuid.NameSpaceOID, []byte(data))
	return hash.String()
}

// Helper function to marshal JSON safely
func MarshalJSON(v interface{}) (json.RawMessage, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return json.RawMessage(data), nil
}
