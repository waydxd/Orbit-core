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
	"github.com/hibiken/asynq"
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
	return NewService(nil, logger.New(), repo, nil, nil, nil)
}

func newTestServiceWithEnqueuer(repo Repository, enqueuer TaskEnqueuer, canceller TaskCanceller) *Service {
	return NewService(nil, logger.New(), repo, nil, enqueuer, canceller)
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
	subID := "sub-1"
	mock := &mockRepo{
		getSubResp: &models.EventSubscription{ID: subID, UserID: "user-1", EventID: "evt-1"},
	}
	enq := &mockEnqueuer{}
	svc := newTestServiceWithEnqueuer(mock, enq, nil)

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
	if !enq.EnqueueCalled {
		t.Fatal("expected Asynq task to be enqueued")
	}
	if !mock.UpdateJobIDCalled {
		t.Fatal("expected UpdateSubscriptionJobID to be called with the task ID")
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

func TestHandleSubscribe_NoEnqueuerStillSucceeds(t *testing.T) {
	// When enqueuer is nil (Redis unavailable), the subscription should still be saved.
	subID := "sub-no-redis"
	mock := &mockRepo{
		getSubResp: &models.EventSubscription{ID: subID},
	}
	svc := newTestService(mock) // no enqueuer

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
		t.Fatalf("expected 201 even without enqueuer, got %d: %s", rr.Code, rr.Body.String())
	}
	if !mock.CreateSubCalled {
		t.Fatal("expected CreateSubscription to be called")
	}
}

func TestHandleUnsubscribe_Success(t *testing.T) {
	jobID := "task-abc"
	mock := &mockRepo{
		getSubResp: &models.EventSubscription{
			ID: "sub-1", UserID: "user-1", EventID: "evt-1", JobID: &jobID, Status: StatusPending,
		},
	}
	canceller := &mockCanceller{}
	svc := newTestServiceWithEnqueuer(mock, nil, canceller)

	req := httptest.NewRequest(http.MethodDelete, "/events/evt-1/notify", nil)
	req = withUserID(req, "user-1")
	req = mux.SetURLVars(req, map[string]string{"id": "evt-1"})
	rr := httptest.NewRecorder()

	svc.handleUnsubscribe(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if !canceller.DeleteCalled {
		t.Fatal("expected Asynq task to be cancelled")
	}
	if canceller.DeletedTaskID != jobID {
		t.Errorf("cancelled wrong task ID: got %q, want %q", canceller.DeletedTaskID, jobID)
	}
	if !mock.MarkStatusCalled || mock.MarkStatusValue != StatusCancelled {
		t.Fatalf("expected MarkSubscriptionStatus('cancelled'), got called=%v value=%q",
			mock.MarkStatusCalled, mock.MarkStatusValue)
	}
}

func TestHandleUnsubscribe_NoCanceller(t *testing.T) {
	// When canceller is nil, unsubscribe should still mark the subscription as cancelled.
	jobID := "task-xyz"
	mock := &mockRepo{
		getSubResp: &models.EventSubscription{
			ID: "sub-2", JobID: &jobID, Status: StatusPending,
		},
	}
	svc := newTestService(mock) // no canceller

	req := httptest.NewRequest(http.MethodDelete, "/events/evt-1/notify", nil)
	req = withUserID(req, "user-1")
	req = mux.SetURLVars(req, map[string]string{"id": "evt-1"})
	rr := httptest.NewRecorder()

	svc.handleUnsubscribe(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if !mock.MarkStatusCalled {
		t.Fatal("expected MarkSubscriptionStatus to be called")
	}
}

// ===== Worker (Asynq handler) tests using mock FCM sender =====

func TestWorker_HandleSendNotification_Success(t *testing.T) {
	sub := &models.EventSubscription{
		ID:      "sub-1",
		UserID:  "user-1",
		EventID: "evt-1",
	}
	token := &models.DeviceToken{
		ID:       "dt-1",
		UserID:   "user-1",
		Token:    "fcm-token-abc",
		Platform: "android",
	}

	mockFCM := &mockFCMClient{}
	repo := &mockRepo{
		tokensResp: []*models.DeviceToken{token},
	}

	w := &Worker{repo: repo, fcm: nil, logger: logger.New()}
	w.sendFn = mockFCM.send

	payload, _ := json.Marshal(SendNotificationPayload{
		UserID:  sub.UserID,
		EventID: sub.EventID,
		SubID:   sub.ID,
	})
	task := newAsynqTaskForTest(TaskTypeSendNotification, payload)

	err := w.HandleSendNotification(context.Background(), task)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !repo.MarkStatusCalled || repo.MarkStatusValue != StatusSent {
		t.Fatalf("expected MarkSubscriptionStatus('sent'), got called=%v value=%q",
			repo.MarkStatusCalled, repo.MarkStatusValue)
	}
	if mockFCM.callCount != 1 {
		t.Fatalf("expected 1 FCM send call, got %d", mockFCM.callCount)
	}
}

func TestWorker_HandleSendNotification_InvalidToken(t *testing.T) {
	token := &models.DeviceToken{
		ID:       "dt-2",
		UserID:   "user-2",
		Token:    "invalid-token",
		Platform: "ios",
	}

	mockFCM := &mockFCMClient{returnErr: errInvalidToken}
	repo := &mockRepo{
		tokensResp: []*models.DeviceToken{token},
	}

	w := &Worker{repo: repo, fcm: nil, logger: logger.New()}
	w.sendFn = mockFCM.send

	payload, _ := json.Marshal(SendNotificationPayload{
		UserID:  "user-2",
		EventID: "evt-2",
		SubID:   "sub-2",
	})
	task := newAsynqTaskForTest(TaskTypeSendNotification, payload)

	// Handler should return an error so Asynq can retry
	err := w.HandleSendNotification(context.Background(), task)
	if err == nil {
		t.Fatal("expected error when FCM send fails, got nil")
	}

	// Give the async goroutine a moment to run
	time.Sleep(20 * time.Millisecond)

	if !repo.DeleteTokenCalled {
		t.Fatal("expected DeleteDeviceToken to be called for invalid token")
	}
}

// newAsynqTaskForTest creates a minimal *asynq.Task for use in handler unit tests.
func newAsynqTaskForTest(typeName string, payload []byte) *asynq.Task {
	return asynq.NewTask(typeName, payload)
}

