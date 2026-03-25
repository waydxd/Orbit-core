package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/logger"
	"github.com/waydxd/Orbit-core/pkg/middleware"
)

// ===== Trigger time calculation tests =====

func TestCalcTriggerTime(t *testing.T) {
	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	eventStart := time.Date(2026, 1, 10, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		eventStart    time.Time
		offsetMinutes int
		wantTrigger   time.Time
		wantFuture    bool
	}{
		{
			name:          "default offset -15 minutes",
			eventStart:    eventStart,
			offsetMinutes: -NotifyOffsetMinutes,
			wantTrigger:   time.Date(2026, 1, 10, 13, 45, 0, 0, time.UTC),
			wantFuture:    true,
		},
		{
			name:          "custom offset -30 minutes",
			eventStart:    eventStart,
			offsetMinutes: -30,
			wantTrigger:   time.Date(2026, 1, 10, 13, 30, 0, 0, time.UTC),
			wantFuture:    true,
		},
		{
			name:          "past event gives past trigger",
			eventStart:    now.Add(-2 * time.Hour),
			offsetMinutes: -NotifyOffsetMinutes,
			wantTrigger:   now.Add(-2*time.Hour - 15*time.Minute),
			wantFuture:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.eventStart.UTC().Add(time.Duration(tt.offsetMinutes) * time.Minute)
			if !got.Equal(tt.wantTrigger) {
				t.Errorf("trigger time = %v, want %v", got, tt.wantTrigger)
			}
			isFuture := got.After(now)
			if isFuture != tt.wantFuture {
				t.Errorf("isFuture = %v, want %v", isFuture, tt.wantFuture)
			}
		})
	}
}

// ===== HTTP handler tests using mock repo =====

func newTestService(repo Repository) *Service {
	return NewService(nil, logger.New(), repo, nil)
}

func withUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
	return r.WithContext(ctx)
}

func TestHandleRegisterToken_Success(t *testing.T) {
	mock := &mockRepo{}
	svc := newTestService(mock)

	body, _ := json.Marshal(map[string]string{"token": "tok1", "platform": "android"})
	req := httptest.NewRequest(http.MethodPost, "/fcm/token", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	rr := httptest.NewRecorder()

	svc.handleRegisterToken(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if !mock.UpsertCalled {
		t.Fatal("expected UpsertDeviceToken to be called")
	}
}

func TestHandleRegisterToken_MissingToken(t *testing.T) {
	mock := &mockRepo{}
	svc := newTestService(mock)

	body, _ := json.Marshal(map[string]string{"token": "", "platform": "ios"})
	req := httptest.NewRequest(http.MethodPost, "/fcm/token", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	rr := httptest.NewRecorder()

	svc.handleRegisterToken(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleRegisterToken_InvalidPlatform(t *testing.T) {
	mock := &mockRepo{}
	svc := newTestService(mock)

	body, _ := json.Marshal(map[string]string{"token": "tok1", "platform": "windows"})
	req := httptest.NewRequest(http.MethodPost, "/fcm/token", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	rr := httptest.NewRecorder()

	svc.handleRegisterToken(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleSubscribe_Success(t *testing.T) {
	mock := &mockRepo{}
	svc := newTestService(mock)

	future := time.Now().UTC().Add(2 * time.Hour)
	body, _ := json.Marshal(map[string]interface{}{
		"event_start_at": future.Format(time.RFC3339),
	})
	req := httptest.NewRequest(http.MethodPost, "/events/evt-1/notify", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	req = mux.SetURLVars(req, map[string]string{"id": "evt-1"})
	rr := httptest.NewRecorder()

	svc.handleSubscribe(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if !mock.CreateSubCalled {
		t.Fatal("expected CreateSubscription to be called")
	}
}

func TestHandleSubscribe_PastEvent(t *testing.T) {
	mock := &mockRepo{}
	svc := newTestService(mock)

	past := time.Now().UTC().Add(-1 * time.Hour)
	body, _ := json.Marshal(map[string]interface{}{
		"event_start_at": past.Format(time.RFC3339),
	})
	req := httptest.NewRequest(http.MethodPost, "/events/evt-1/notify", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	req = mux.SetURLVars(req, map[string]string{"id": "evt-1"})
	rr := httptest.NewRecorder()

	svc.handleSubscribe(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for past event, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSubscribe_Duplicate(t *testing.T) {
	mock := &mockRepo{existsResp: true}
	svc := newTestService(mock)

	future := time.Now().UTC().Add(2 * time.Hour)
	body, _ := json.Marshal(map[string]interface{}{
		"event_start_at": future.Format(time.RFC3339),
	})
	req := httptest.NewRequest(http.MethodPost, "/events/evt-1/notify", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	req = mux.SetURLVars(req, map[string]string{"id": "evt-1"})
	rr := httptest.NewRecorder()

	svc.handleSubscribe(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleUnsubscribe_Success(t *testing.T) {
	mock := &mockRepo{}
	svc := newTestService(mock)

	req := httptest.NewRequest(http.MethodDelete, "/events/evt-1/notify", nil)
	req = withUserID(req, "user-1")
	req = mux.SetURLVars(req, map[string]string{"id": "evt-1"})
	rr := httptest.NewRecorder()

	svc.handleUnsubscribe(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if !mock.DeleteSubCalled {
		t.Fatal("expected DeleteSubscription to be called")
	}
}

// ===== Worker loop tests using mock FCM sender =====

func TestWorker_ProcessesPendingSubscriptions(t *testing.T) {
	now := time.Now().UTC()
	sub := &models.EventSubscription{
		ID:          "sub-1",
		UserID:      "user-1",
		EventID:     "evt-1",
		TriggerTime: now.Add(-1 * time.Minute),
	}
	token := &models.DeviceToken{
		ID:       "dt-1",
		UserID:   "user-1",
		Token:    "fcm-token-abc",
		Platform: "android",
	}

	mockFCM := &mockFCMClient{}
	repo := &mockRepo{
		pendingSubsResp: []*models.EventSubscription{sub},
		tokensResp:      []*models.DeviceToken{token},
	}

	w := &Worker{repo: repo, fcm: nil, logger: logger.New()}
	// Inject mock FCM via the send function directly
	w.sendFn = mockFCM.send

	w.run(context.Background())

	if !repo.MarkSentCalled {
		t.Fatal("expected MarkSubscriptionSent to be called")
	}
	if mockFCM.callCount != 1 {
		t.Fatalf("expected 1 FCM send call, got %d", mockFCM.callCount)
	}
}

func TestWorker_InvalidToken_Deleted(t *testing.T) {
	now := time.Now().UTC()
	sub := &models.EventSubscription{
		ID:          "sub-2",
		UserID:      "user-2",
		EventID:     "evt-2",
		TriggerTime: now.Add(-1 * time.Minute),
	}
	token := &models.DeviceToken{
		ID:       "dt-2",
		UserID:   "user-2",
		Token:    "invalid-token",
		Platform: "ios",
	}

	mockFCM := &mockFCMClient{returnErr: errInvalidToken}
	repo := &mockRepo{
		pendingSubsResp: []*models.EventSubscription{sub},
		tokensResp:      []*models.DeviceToken{token},
	}

	w := &Worker{repo: repo, fcm: nil, logger: logger.New()}
	w.sendFn = mockFCM.send

	w.run(context.Background())

	// Give async goroutine a moment to run
	time.Sleep(20 * time.Millisecond)

	if !repo.DeleteTokenCalled {
		t.Fatal("expected DeleteDeviceToken to be called for invalid token")
	}
}
