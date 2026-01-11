package chat

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockChatRepository is a mock for the IChatRepository interface.
type MockChatRepository struct {
	messages []Message // Assuming Message struct exists
}

// MockMessage represents a dummy message for testing.
type Message struct {
	ID            string    `json:"id"`
	ConversationID string    `json:"conversationId"`
	SenderID      string    `json:"senderId"`
	Content       string    `json:"content"`
	Timestamp     time.Time `json:"timestamp"`
	IsAgent       bool      `json:"isAgent"`
}

// NewMockChatRepository creates a new mock repository.
func NewMockChatRepository() *MockChatRepository {
	return &MockChatRepository{
		messages: []Message{},
	}
}

// SaveMessage simulates saving a message.
func (r *MockChatRepository) SaveMessage(ctx context.Context, message *Message) (*Message, error) {
	message.ID = "mock-msg-id-" + time.Now().Format("20060102150405")
	message.Timestamp = time.Now()
	r.messages = append(r.messages, *message)
	return message, nil
}

// GetMessagesByConversationID simulates retrieving messages for a conversation.
func (r *MockChatRepository) GetMessagesByConversationID(ctx context.Context, conversationID string) ([]Message, error) {
	var result []Message
	for _, msg := range r.messages {
		if msg.ConversationID == conversationID {
			result = append(result, msg)
		}
	}
	if len(result) == 0 {
		return nil, errors.New("no messages found") // Simulate not found
	}
	return result, nil
}

// DeleteMessagesByConversationID simulates deleting messages.
func (r *MockChatRepository) DeleteMessagesByConversationID(ctx context.Context, conversationID string) error {
	initialCount := len(r.messages)
	var updatedMessages []Message
	for _, msg := range r.messages {
		if msg.ConversationID != conversationID {
			updatedMessages = append(updatedMessages, msg)
		}
	}
	r.messages = updatedMessages
	if len(r.messages) == initialCount {
		return errors.New("no messages deleted, conversation ID not found")
	}
	return nil
}

// --- Test Cases for Service ---

// ChatService represents the service layer for chat operations.
type ChatService struct {
	repo IChatRepository // Assuming IChatRepository interface exists
}

// NewChatService creates a new ChatService.
func NewChatService(repo IChatRepository) *ChatService {
	return &ChatService{
		repo: repo,
	}
}

func TestChatService_SendMessage(t *testing.T) {
	mockRepo := NewMockChatRepository()
	svc := NewChatService(mockRepo)
	ctx := context.Background()

	conversationID := "conv123"
	senderID := "userABC"
	content := "Hello, Chatbot!"

	// Test case 1: Successful message sending
	message, err := svc.SendMessage(ctx, conversationID, senderID, content, false) // false indicates not an agent message
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if message == nil {
		t.Fatal("SendMessage returned nil message")
	}
	if message.ConversationID != conversationID || message.SenderID != senderID || message.Content != content || message.IsAgent {
		t.Errorf("SendMessage returned message with incorrect data")
	}
	if message.ID == "" || message.Timestamp.IsZero() {
		t.Error("SendMessage did not assign ID or Timestamp")
	}
	if len(mockRepo.messages) != 1 {
		t.Errorf("Expected 1 message in repo, got %d", len(mockRepo.messages))
	}

	// Test case 2: Repository error during save
	mockRepo.SaveMessage = func(ctx context.Context, msg *Message) (*Message, error) {
		return nil, errors.New("repo save error")
	}
	_, err = svc.SendMessage(ctx, conversationID, senderID, "another message", false)
	if err == nil {
		t.Error("SendMessage should return an error on repo save failure, but got nil")
	}
	if err.Error() != "repo save error" {
		t.Errorf("SendMessage returned wrong error message: got %q, want %q", err.Error(), "repo save error")
	}
}

func TestChatService_GetConversationHistory(t *testing.T) {
	mockRepo := NewMockChatRepository()
	svc := NewChatService(mockRepo)
	ctx := context.Background()
	conversationID := "conv456"

	// Populate repo with some messages
	mockRepo.messages = []MockChatRepository.Message{
		{ID: "m1", ConversationID: conversationID, SenderID: "userXYZ", Content: "First msg", IsAgent: false},
		{ID: "m2", ConversationID: conversationID, SenderID: "agentBot", Content: "Reply msg", IsAgent: true},
		{ID: "m3", ConversationID: "otherConv", SenderID: "userXYZ", Content: "Other conv msg", IsAgent: false},
	}

	// Test case 1: Conversation found
	messages, err := svc.GetConversationHistory(ctx, conversationID)
	if err != nil {
		t.Fatalf("GetConversationHistory(%q) failed: %v", conversationID, err)
	}
	if len(messages) != 2 {
		t.Errorf("GetConversationHistory(%q) returned %d messages, want 2", conversationID, len(messages))
	}

	// Test case 2: Conversation not found
	nonExistentConvID := "non-existent-conv"
	_, err = svc.GetConversationHistory(ctx, nonExistentConvID)
	if err == nil {
		t.Errorf("GetConversationHistory(%q) should return an error, but got nil", nonExistentConvID)
	}
	if err != nil && err.Error() != "no messages found" { // Expecting error from mock repo
		t.Errorf("GetConversationHistory(%q) returned unexpected error: %v", nonExistentConvID, err)
	}

	// Test case 3: Repository error
	mockRepo.GetMessagesByConversationID = func(ctx context.Context, conversationID string) ([]Message, error) {
		return nil, errors.New("repo get error")
	}
	_, err = svc.GetConversationHistory(ctx, conversationID)
	if err == nil {
		t.Error("GetConversationHistory should return an error when repository fails, but got nil")
	}
	if err.Error() != "repo get error" {
		t.Errorf("GetConversationHistory returned wrong error message: got %q, want %q", err.Error(), "repo get error")
	}
}

// Add tests for other service methods like:
// - GetConversations (if applicable)
// - DeleteConversation (if applicable)
// - Handling different message types, errors, etc.

// NOTE: The 'Message' struct and 'IChatRepository' interface need to be defined
// and imported from the 'chat' package.
// This mock implementation assumes their basic structure and methods.
