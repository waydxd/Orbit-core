package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Repository defines the interface for chat data operations
type Repository interface {
	// CreateConversation GetConversationByID GetConversationByCorrelationID ListConversationsByUser UpdateConversationStatus
	// Conversation operations
	CreateConversation(ctx context.Context, userID string, correlationID string) (*models.Conversation, error)
	GetConversationByID(ctx context.Context, conversationID string) (*models.Conversation, error)
	GetConversationByCorrelationID(ctx context.Context, correlationID string) (*models.Conversation, error)
	ListConversationsByUser(ctx context.Context, userID string, limit int) ([]*models.Conversation, error)
	UpdateConversationStatus(ctx context.Context, conversationID string, status string) error

	// CreateMessage GetMessagesByConversation
	// Message operations
	CreateMessage(ctx context.Context, msg *models.ChatMessage) (*models.ChatMessage, error)
	GetMessagesByConversation(ctx context.Context, conversationID string) ([]*models.ChatMessage, error)

	// CreatePendingAction GetPendingActionByID GetPendingActionsByConversation UpdatePendingActionStatus GetExpiredActions
	// Pending action operations
	CreatePendingAction(ctx context.Context, action *models.PendingAction) (*models.PendingAction, error)
	GetPendingActionByID(ctx context.Context, actionID string) (*models.PendingAction, error)
	GetPendingActionsByConversation(ctx context.Context, conversationID string) ([]*models.PendingAction, error)
	UpdatePendingActionStatus(ctx context.Context, actionID string, status string, version int, errorMsg string) error
	GetExpiredActions(ctx context.Context) ([]*models.PendingAction, error)

	// CreateToolLog GetToolLogsByConversation GetToolLogsByPendingAction
	// Tool log operations
	CreateToolLog(ctx context.Context, log *models.AgentToolLog) (*models.AgentToolLog, error)
	GetToolLogsByConversation(ctx context.Context, conversationID string) ([]*models.AgentToolLog, error)
	GetToolLogsByPendingAction(ctx context.Context, pendingActionID string) ([]*models.AgentToolLog, error)
}

// MongoRepository implements Repository using MongoDB
type MongoRepository struct {
	client *mongo.Client
	dbName string
}

// NewMongoRepository creates a new Mongo repository and ensures necessary indexes
func NewMongoRepository(ctx context.Context, client *mongo.Client, dbName string) (Repository, error) {
	repo := &MongoRepository{
		client: client,
		dbName: dbName,
	}

	// Ensure indexes for pending_actions collection
	pendingActionsCollection := client.Database(dbName).Collection("pending_actions")
	// Create a compound index on {status: 1, expires_at: 1} to optimize queries
	// that filter by status and expiration time.
	_, err := pendingActionsCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "status", Value: 1}, {Key: "expires_at", Value: 1}},
		Options: options.Index(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create index on pending_actions: %w", err)
	}

	return repo, nil
}

// CreateConversation creates a new conversation
func (r *MongoRepository) CreateConversation(ctx context.Context, userID string, correlationID string) (*models.Conversation, error) {
	collection := r.client.Database(r.dbName).Collection("conversations")

	conv := &models.Conversation{
		ID:            uuid.New().String(),
		UserID:        userID,
		CorrelationID: correlationID,
		Status:        "active",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	_, err := collection.InsertOne(ctx, conv)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	return conv, nil
}

// GetConversationByID retrieves a conversation by ID
func (r *MongoRepository) GetConversationByID(ctx context.Context, conversationID string) (*models.Conversation, error) {
	collection := r.client.Database(r.dbName).Collection("conversations")

	var conv models.Conversation
	err := collection.FindOne(ctx, bson.M{"id": conversationID}).Decode(&conv)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("conversation not found")
		}
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	return &conv, nil
}

// GetConversationByCorrelationID retrieves a conversation by correlation ID
func (r *MongoRepository) GetConversationByCorrelationID(ctx context.Context, correlationID string) (*models.Conversation, error) {
	collection := r.client.Database(r.dbName).Collection("conversations")

	var conv models.Conversation
	err := collection.FindOne(ctx, bson.M{"correlationid": correlationID}).Decode(&conv)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("conversation not found")
		}
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	return &conv, nil
}

// ListConversationsByUser retrieves conversations for a user
func (r *MongoRepository) ListConversationsByUser(ctx context.Context, userID string, limit int) ([]*models.Conversation, error) {
	collection := r.client.Database(r.dbName).Collection("conversations")

	opts := options.Find().SetSort(bson.D{{Key: "createdat", Value: -1}}).SetLimit(int64(limit))
	cursor, err := collection.Find(ctx, bson.M{"userid": userID}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			log.Printf("failed to close cursor: %v", err)
		}
	}(cursor, ctx)

	var conversations []*models.Conversation
	for cursor.Next(ctx) {
		var conv models.Conversation
		if err := cursor.Decode(&conv); err != nil {
			return nil, fmt.Errorf("failed to decode conversation: %w", err)
		}
		conversations = append(conversations, &conv)
	}

	return conversations, nil
}

// UpdateConversationStatus updates the status of a conversation
func (r *MongoRepository) UpdateConversationStatus(ctx context.Context, conversationID string, status string) error {
	collection := r.client.Database(r.dbName).Collection("conversations")

	_, err := collection.UpdateOne(
		ctx,
		bson.M{"id": conversationID},
		bson.M{"$set": bson.M{"status": status, "updatedat": time.Now()}},
	)
	if err != nil {
		return fmt.Errorf("failed to update conversation status: %w", err)
	}
	return nil
}

// CreateMessage creates a new chat message
func (r *MongoRepository) CreateMessage(ctx context.Context, msg *models.ChatMessage) (*models.ChatMessage, error) {
	collection := r.client.Database(r.dbName).Collection("chat_messages")

	msg.ID = uuid.New().String()
	msg.CreatedAt = time.Now()

	_, err := collection.InsertOne(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	return msg, nil
}

// GetMessagesByConversation retrieves messages for a conversation
func (r *MongoRepository) GetMessagesByConversation(ctx context.Context, conversationID string) ([]*models.ChatMessage, error) {
	collection := r.client.Database(r.dbName).Collection("chat_messages")

	opts := options.Find().SetSort(bson.D{{Key: "createdat", Value: 1}})
	cursor, err := collection.Find(ctx, bson.M{"conversationid": conversationID}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			panic(err)
		}
	}(cursor, ctx)

	var messages []*models.ChatMessage
	for cursor.Next(ctx) {
		var msg models.ChatMessage
		if err := cursor.Decode(&msg); err != nil {
			return nil, fmt.Errorf("failed to decode message: %w", err)
		}
		messages = append(messages, &msg)
	}

	return messages, nil
}

// CreatePendingAction creates a new pending action
func (r *MongoRepository) CreatePendingAction(ctx context.Context, action *models.PendingAction) (*models.PendingAction, error) {
	collection := r.client.Database(r.dbName).Collection("pending_actions")

	action.ID = uuid.New().String()
	action.CreatedAt = time.Now()
	action.UpdatedAt = time.Now()

	_, err := collection.InsertOne(ctx, action)
	if err != nil {
		return nil, fmt.Errorf("failed to create pending action: %w", err)
	}

	return action, nil
}

// GetPendingActionByID retrieves a pending action by action ID
func (r *MongoRepository) GetPendingActionByID(ctx context.Context, actionID string) (*models.PendingAction, error) {
	collection := r.client.Database(r.dbName).Collection("pending_actions")

	var pa models.PendingAction
	err := collection.FindOne(ctx, bson.M{"actionid": actionID}).Decode(&pa)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("pending action not found")
		}
		return nil, fmt.Errorf("failed to get pending action: %w", err)
	}

	return &pa, nil
}

// GetPendingActionsByConversation retrieves pending actions for a conversation
func (r *MongoRepository) GetPendingActionsByConversation(ctx context.Context, conversationID string) ([]*models.PendingAction, error) {
	collection := r.client.Database(r.dbName).Collection("pending_actions")

	opts := options.Find().SetSort(bson.D{{Key: "createdat", Value: -1}})
	cursor, err := collection.Find(ctx, bson.M{"conversationid": conversationID}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending actions: %w", err)
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			panic(err)
		}
	}(cursor, ctx)

	var actions []*models.PendingAction
	for cursor.Next(ctx) {
		var pa models.PendingAction
		if err := cursor.Decode(&pa); err != nil {
			return nil, fmt.Errorf("failed to decode pending action: %w", err)
		}
		actions = append(actions, &pa)
	}

	return actions, nil
}

// UpdatePendingActionStatus updates a pending action with optimistic locking
func (r *MongoRepository) UpdatePendingActionStatus(ctx context.Context, actionID string, status string, version int, errorMsg string) error {
	collection := r.client.Database(r.dbName).Collection("pending_actions")

	result, err := collection.UpdateOne(
		ctx,
		bson.M{"actionid": actionID, "version": version},
		bson.M{"$set": bson.M{
			"status":       status,
			"version":      version + 1,
			"errormessage": errorMsg,
			"updatedat":    time.Now(),
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to update pending action: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("version mismatch or action not found: optimistic lock failed")
	}

	return nil
}

// GetExpiredActions retrieves expired pending actions
func (r *MongoRepository) GetExpiredActions(ctx context.Context) ([]*models.PendingAction, error) {
	collection := r.client.Database(r.dbName).Collection("pending_actions")

	cursor, err := collection.Find(ctx, bson.M{
		"status":     "pending",
		"expires_at": bson.M{"$lt": time.Now()},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get expired actions: %w", err)
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			panic(fmt.Sprintf("failed to close cursor: %v", err))
		}
	}(cursor, ctx)

	var actions []*models.PendingAction
	for cursor.Next(ctx) {
		var pa models.PendingAction
		if err := cursor.Decode(&pa); err != nil {
			return nil, fmt.Errorf("failed to decode expired action: %w", err)
		}
		actions = append(actions, &pa)
	}

	return actions, nil
}

// CreateToolLog creates an agent tool log
func (r *MongoRepository) CreateToolLog(ctx context.Context, log *models.AgentToolLog) (*models.AgentToolLog, error) {
	collection := r.client.Database(r.dbName).Collection("agent_tool_logs")

	log.ID = uuid.New().String()
	log.CreatedAt = time.Now()

	if _, err := collection.InsertOne(ctx, log); err != nil {
		return nil, fmt.Errorf("failed to create tool log: %w", err)
	}

	return log, nil
}

// GetToolLogsByConversation retrieves tool logs for a conversation
func (r *MongoRepository) GetToolLogsByConversation(ctx context.Context, conversationID string) ([]*models.AgentToolLog, error) {
	collection := r.client.Database(r.dbName).Collection("agent_tool_logs")

	opts := options.Find().SetSort(bson.D{{Key: "createdat", Value: 1}})
	cursor, err := collection.Find(ctx, bson.M{"conversationid": conversationID}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool logs: %w", err)
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			panic(err)
		}
	}(cursor, ctx)

	var logs []*models.AgentToolLog
	for cursor.Next(ctx) {
		var tl models.AgentToolLog
		if err := cursor.Decode(&tl); err != nil {
			return nil, fmt.Errorf("failed to decode tool log: %w", err)
		}
		logs = append(logs, &tl)
	}

	return logs, nil
}

// GetToolLogsByPendingAction retrieves tool logs for a pending action
func (r *MongoRepository) GetToolLogsByPendingAction(ctx context.Context, pendingActionID string) ([]*models.AgentToolLog, error) {
	collection := r.client.Database(r.dbName).Collection("agent_tool_logs")

	opts := options.Find().SetSort(bson.D{{Key: "createdat", Value: 1}})
	cursor, err := collection.Find(ctx, bson.M{"pendingactionid": pendingActionID}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool logs: %w", err)
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			panic(err)
		}
	}(cursor, ctx)

	var logs []*models.AgentToolLog
	for cursor.Next(ctx) {
		var tl models.AgentToolLog
		if err := cursor.Decode(&tl); err != nil {
			return nil, fmt.Errorf("failed to decode tool log: %w", err)
		}
		logs = append(logs, &tl)
	}

	return logs, nil
}

// GenerateActionID Helper function to generate action ID
func GenerateActionID() string {
	return fmt.Sprintf("action_%s", uuid.New().String())
}

// GenerateCorrelationID Helper function to generate correlation ID
func GenerateCorrelationID() string {
	return uuid.New().String()
}

// GenerateIdempotencyKey Helper function to generate idempotency key
func GenerateIdempotencyKey(userID, conversationID, actionType string, timestamp int64) string {
	data := fmt.Sprintf("%s:%s:%s:%d", userID, conversationID, actionType, timestamp)
	hash := uuid.NewSHA1(uuid.NameSpaceOID, []byte(data))
	return hash.String()
}

// MarshalJSON Helper function to marshal JSON safely
func MarshalJSON(v interface{}) (json.RawMessage, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return json.RawMessage(data), nil
}
