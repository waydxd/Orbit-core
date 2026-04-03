package chat

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
	"github.com/waydxd/Orbit-core/pkg/middleware"
)

type mockRepo struct {
	conversation *models.Conversation
	getConvErr   error
}

func (m *mockRepo) CreateConversation(_ context.Context, userID string, correlationID string) (*models.Conversation, error) {
	return &models.Conversation{ID: "11111111-1111-1111-1111-111111111111", UserID: userID, CorrelationID: correlationID, Status: "active"}, nil
}

func (m *mockRepo) GetConversationByID(_ context.Context, _ string) (*models.Conversation, error) {
	if m.getConvErr != nil {
		return nil, m.getConvErr
	}
	if m.conversation == nil {
		return nil, errors.New("conversation not found")
	}
	return m.conversation, nil
}

func (m *mockRepo) GetConversationByCorrelationID(_ context.Context, _ string) (*models.Conversation, error) {
	return nil, errors.New("not implemented")
}

func (m *mockRepo) ListConversationsByUser(_ context.Context, _ string, _ int) ([]*models.Conversation, error) {
	return nil, nil
}

func (m *mockRepo) UpdateConversationStatus(_ context.Context, _ string, _ string) error { return nil }

func (m *mockRepo) CreateMessage(_ context.Context, msg *models.ChatMessage) (*models.ChatMessage, error) {
	return msg, nil
}

func (m *mockRepo) GetMessagesByConversation(_ context.Context, _ string) ([]*models.ChatMessage, error) {
	return []*models.ChatMessage{}, nil
}

func (m *mockRepo) CreatePendingAction(_ context.Context, action *models.PendingAction) (*models.PendingAction, error) {
	return action, nil
}

func (m *mockRepo) GetPendingActionByID(_ context.Context, _ string) (*models.PendingAction, error) {
	return nil, errors.New("not found")
}

func (m *mockRepo) GetPendingActionsByConversation(_ context.Context, _ string) ([]*models.PendingAction, error) {
	return []*models.PendingAction{}, nil
}

func (m *mockRepo) UpdatePendingActionStatus(_ context.Context, _ string, _ string, _ int, _ string) error {
	return nil
}

func (m *mockRepo) GetExpiredActions(_ context.Context) ([]*models.PendingAction, error) {
	return []*models.PendingAction{}, nil
}

func (m *mockRepo) CreateToolLog(_ context.Context, l *models.AgentToolLog) (*models.AgentToolLog, error) {
	return l, nil
}

func (m *mockRepo) GetToolLogsByConversation(_ context.Context, _ string) ([]*models.AgentToolLog, error) {
	return []*models.AgentToolLog{}, nil
}

func (m *mockRepo) GetToolLogsByPendingAction(_ context.Context, _ string) ([]*models.AgentToolLog, error) {
	return []*models.AgentToolLog{}, nil
}

func withUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
	return r.WithContext(ctx)
}

func TestHandlePostMessage_RequiresConversationID(t *testing.T) {
	svc := NewService(&config.Config{}, logger.New(), &mockRepo{}, nil, nil)
	router := mux.NewRouter()
	svc.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/chat/messages", strings.NewReader(`{"message":"hello"}`))
	req = withUserID(req, "user-1")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}

	var payload ErrorResponse
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if payload.Code != "missing_conversation_id" {
		t.Fatalf("expected missing_conversation_id, got %s", payload.Code)
	}
}

func TestHandlePostMessage_UnknownConversationReturns404(t *testing.T) {
	svc := NewService(&config.Config{}, logger.New(), &mockRepo{getConvErr: errors.New("not found")}, nil, nil)
	router := mux.NewRouter()
	svc.RegisterRoutes(router)

	body := `{"message":"hello","conversation_id":"11111111-1111-1111-1111-111111111111"}`
	req := httptest.NewRequest(http.MethodPost, "/chat/messages", strings.NewReader(body))
	req = withUserID(req, "user-1")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, res.Code)
	}
}

func TestHandleCreateConversation_Created(t *testing.T) {
	svc := NewService(&config.Config{}, logger.New(), &mockRepo{}, nil, nil)
	router := mux.NewRouter()
	svc.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/chat/conversations", nil)
	req = withUserID(req, "user-1")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, res.Code)
	}

	var payload CreateConversationResponse
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if payload.ConversationID == "" {
		t.Fatal("expected conversation id")
	}
}

func TestValidateActionForConfirmation_ExpiredActionRejected(t *testing.T) {
	repo := &mockRepo{}
	svc := NewService(&config.Config{}, logger.New(), repo, nil, nil)
	action := &models.PendingAction{
		ActionID:       "11111111-1111-1111-1111-111111111111",
		ActionType:     "create_event",
		IdempotencyKey: "idem-1",
		Status:         "pending",
		Version:        0,
		ExpiresAt:      time.Now().Add(-1 * time.Minute),
	}

	err := svc.validateActionForConfirmation(context.Background(), action, "idem-1")
	if !errors.Is(err, ErrActionExpired) {
		t.Fatalf("expected ErrActionExpired, got %v", err)
	}
}
