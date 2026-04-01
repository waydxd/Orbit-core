package notification

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

// ===== Event subscribe tests =====

func TestHandleSubscribeEvent_Success(t *testing.T) {
	mock := &mockRepo{
		createSubID: "sub-1",
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

	svc.handleSubscribeEvent(rr, req)

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
	if mock.UpdateJobIDValue != "mock-task-id-123" {
		t.Fatalf("expected task ID to be persisted, got %q", mock.UpdateJobIDValue)
	}
}

func TestHandleSubscribeEvent_PastEvent(t *testing.T) {
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

	svc.handleSubscribeEvent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for past event, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSubscribeEvent_Duplicate(t *testing.T) {
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

	svc.handleSubscribeEvent(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSubscribeEvent_NoEnqueuerStillSucceeds(t *testing.T) {
	mock := &mockRepo{
		createSubID: "sub-no-redis",
	}
	svc := newTestService(mock)

	future := time.Now().UTC().Add(2 * time.Hour)
	body, _ := json.Marshal(map[string]interface{}{
		"event_start_at": future.Format(time.RFC3339),
	})
	req := httptest.NewRequest(http.MethodPost, "/events/evt-1/notify", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	req = mux.SetURLVars(req, map[string]string{"id": "evt-1"})
	rr := httptest.NewRecorder()

	svc.handleSubscribeEvent(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 even without enqueuer, got %d: %s", rr.Code, rr.Body.String())
	}
	if !mock.CreateSubCalled {
		t.Fatal("expected CreateSubscription to be called")
	}
}

func TestHandleSubscribeEvent_TaskConstructionErrorRollsBack(t *testing.T) {
	oldFactory := makeSendNotificationTask
	defer func() { makeSendNotificationTask = oldFactory }()
	makeSendNotificationTask = func(_ SendNotificationPayload) (*asynq.Task, error) {
		return nil, errors.New("boom")
	}

	mock := &mockRepo{createSubID: "sub-task-fail"}
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

	svc.handleSubscribeEvent(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}
	if !mock.DeleteSubCalled {
		t.Fatal("expected subscription rollback delete after task construction failure")
	}
	if enq.EnqueueCalled {
		t.Fatal("did not expect enqueue on task construction failure")
	}
}

// ===== Event unsubscribe tests =====

func TestHandleUnsubscribeEvent_Success(t *testing.T) {
	jobID := "task-abc"
	mock := &mockRepo{
		getSubResp: &models.EventSubscription{
			ID: "sub-1", UserID: "user-1", EntityID: "evt-1", EntityType: EntityTypeEvent,
			JobID: &jobID, Status: StatusPending,
		},
	}
	canceller := &mockCanceller{}
	svc := newTestServiceWithEnqueuer(mock, nil, canceller)

	req := httptest.NewRequest(http.MethodDelete, "/events/evt-1/notify", nil)
	req = withUserID(req, "user-1")
	req = mux.SetURLVars(req, map[string]string{"id": "evt-1"})
	rr := httptest.NewRecorder()

	svc.handleUnsubscribeEvent(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if !canceller.DeleteCalled {
		t.Fatal("expected Asynq task to be canceled")
	}
	if canceller.DeletedTaskID != jobID {
		t.Errorf("canceled wrong task ID: got %q, want %q", canceller.DeletedTaskID, jobID)
	}
	if !mock.MarkStatusCalled || mock.MarkStatusValue != StatusCancelled {
		t.Fatalf("expected MarkSubscriptionStatus('cancelled'), got called=%v value=%q",
			mock.MarkStatusCalled, mock.MarkStatusValue)
	}
}

func TestHandleUnsubscribeEvent_NoCanceller(t *testing.T) {
	jobID := "task-xyz"
	mock := &mockRepo{
		getSubResp: &models.EventSubscription{
			ID: "sub-2", EntityID: "evt-1", EntityType: EntityTypeEvent,
			JobID: &jobID, Status: StatusPending,
		},
	}
	svc := newTestService(mock)

	req := httptest.NewRequest(http.MethodDelete, "/events/evt-1/notify", nil)
	req = withUserID(req, "user-1")
	req = mux.SetURLVars(req, map[string]string{"id": "evt-1"})
	rr := httptest.NewRecorder()

	svc.handleUnsubscribeEvent(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if !mock.MarkStatusCalled {
		t.Fatal("expected MarkSubscriptionStatus to be called")
	}
}

func TestHandleUnsubscribeEvent_NotFoundIsNoOp(t *testing.T) {
	mock := &mockRepo{getSubErr: sql.ErrNoRows}
	svc := newTestService(mock)

	req := httptest.NewRequest(http.MethodDelete, "/events/evt-1/notify", nil)
	req = withUserID(req, "user-1")
	req = mux.SetURLVars(req, map[string]string{"id": "evt-1"})
	rr := httptest.NewRecorder()

	svc.handleUnsubscribeEvent(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if mock.DeleteSubCalled {
		t.Fatal("did not expect DeleteSubscription on missing active row")
	}
}

// ===== Task subscribe tests =====

func TestHandleSubscribeTask_Success(t *testing.T) {
	mock := &mockRepo{createSubID: "sub-t1"}
	enq := &mockEnqueuer{}
	svc := newTestServiceWithEnqueuer(mock, enq, nil)

	future := time.Now().UTC().Add(2 * time.Hour)
	body, _ := json.Marshal(map[string]interface{}{
		"task_due_at": future.Format(time.RFC3339),
	})
	req := httptest.NewRequest(http.MethodPost, "/tasks/task-1/notify", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	req = mux.SetURLVars(req, map[string]string{"id": "task-1"})
	rr := httptest.NewRecorder()

	svc.handleSubscribeTask(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if !mock.CreateSubCalled {
		t.Fatal("expected CreateSubscription to be called")
	}
	if !enq.EnqueueCalled {
		t.Fatal("expected Asynq task to be enqueued")
	}
}

func TestHandleSubscribeTask_MissingDueAt(t *testing.T) {
	mock := &mockRepo{}
	svc := newTestService(mock)

	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest(http.MethodPost, "/tasks/task-1/notify", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	req = mux.SetURLVars(req, map[string]string{"id": "task-1"})
	rr := httptest.NewRecorder()

	svc.handleSubscribeTask(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleUnsubscribeTask_Success(t *testing.T) {
	jobID := "task-job-1"
	mock := &mockRepo{
		getSubResp: &models.EventSubscription{
			ID: "sub-t1", UserID: "user-1", EntityID: "task-1", EntityType: EntityTypeTask,
			JobID: &jobID, Status: StatusPending,
		},
	}
	canceller := &mockCanceller{}
	svc := newTestServiceWithEnqueuer(mock, nil, canceller)

	req := httptest.NewRequest(http.MethodDelete, "/tasks/task-1/notify", nil)
	req = withUserID(req, "user-1")
	req = mux.SetURLVars(req, map[string]string{"id": "task-1"})
	rr := httptest.NewRecorder()

	svc.handleUnsubscribeTask(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if !canceller.DeleteCalled {
		t.Fatal("expected Asynq task to be canceled")
	}
	if !mock.MarkStatusCalled || mock.MarkStatusValue != StatusCancelled {
		t.Fatalf("expected MarkSubscriptionStatus('cancelled'), got called=%v value=%q",
			mock.MarkStatusCalled, mock.MarkStatusValue)
	}
}

// ===== Worker (Asynq handler) tests using mock FCM sender =====

func TestWorker_HandleSendNotification_EventSuccess(t *testing.T) {
	sub := &models.EventSubscription{
		ID:         "sub-1",
		UserID:     "user-1",
		EntityID:   "evt-1",
		EntityType: EntityTypeEvent,
		Status:     StatusPending,
	}
	//nolint:gosec // G101: False positive - this is test fixture data
	token := &models.DeviceToken{
		ID:       "dt-1",
		UserID:   "user-1",
		Token:    "fcm-token-abc",
		Platform: "android",
	}

	mockFCM := &mockFCMClient{}
	repo := &mockRepo{
		getSubByIDResp: sub,
		tokensResp:     []*models.DeviceToken{token},
	}

	w := &Worker{repo: repo, fcm: nil, logger: logger.New()}
	w.sendFn = mockFCM.send

	payload, _ := json.Marshal(SendNotificationPayload{
		UserID:     sub.UserID,
		EntityID:   sub.EntityID,
		EntityType: sub.EntityType,
		SubID:      sub.ID,
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

func TestWorker_HandleSendNotification_TaskSuccess(t *testing.T) {
	sub := &models.EventSubscription{
		ID:         "sub-t1",
		UserID:     "user-1",
		EntityID:   "task-1",
		EntityType: EntityTypeTask,
		Status:     StatusPending,
	}
	token := &models.DeviceToken{
		ID: "dt-1", UserID: "user-1", Token: "fcm-token-abc", Platform: "android",
	}

	mockFCM := &mockFCMClient{}
	repo := &mockRepo{
		getSubByIDResp: sub,
		tokensResp:     []*models.DeviceToken{token},
	}

	w := &Worker{repo: repo, fcm: nil, logger: logger.New()}
	w.sendFn = mockFCM.send

	payload, _ := json.Marshal(SendNotificationPayload{
		UserID:     sub.UserID,
		EntityID:   sub.EntityID,
		EntityType: sub.EntityType,
		SubID:      sub.ID,
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
}

func TestWorker_HandleSendNotification_BackwardCompatPayload(t *testing.T) {
	sub := &models.EventSubscription{
		ID:         "sub-old",
		UserID:     "user-1",
		EntityID:   "evt-old",
		EntityType: EntityTypeEvent,
		Status:     StatusPending,
	}
	token := &models.DeviceToken{
		ID: "dt-1", UserID: "user-1", Token: "tok", Platform: "ios",
	}

	mockFCM := &mockFCMClient{}
	repo := &mockRepo{
		getSubByIDResp: sub,
		tokensResp:     []*models.DeviceToken{token},
	}

	w := &Worker{repo: repo, fcm: nil, logger: logger.New()}
	w.sendFn = mockFCM.send

	payload, _ := json.Marshal(map[string]string{
		"user_id":  "user-1",
		"event_id": "evt-old",
		"sub_id":   "sub-old",
	})
	task := newAsynqTaskForTest(TaskTypeSendNotification, payload)

	err := w.HandleSendNotification(context.Background(), task)
	if err != nil {
		t.Fatalf("expected no error from backward-compat payload, got: %v", err)
	}
	if mockFCM.callCount != 1 {
		t.Fatalf("expected 1 FCM send, got %d", mockFCM.callCount)
	}
}

func TestWorker_HandleSendNotification_SkipsCancelledSubscription(t *testing.T) {
	repo := &mockRepo{
		getSubByIDResp: &models.EventSubscription{ID: "sub-1", EntityType: EntityTypeEvent, Status: StatusCancelled},
	}
	mockFCM := &mockFCMClient{}
	w := &Worker{repo: repo, fcm: nil, logger: logger.New()}
	w.sendFn = mockFCM.send

	payload, _ := json.Marshal(SendNotificationPayload{
		UserID: "user-1", EntityID: "evt-1", EntityType: EntityTypeEvent, SubID: "sub-1",
	})
	task := newAsynqTaskForTest(TaskTypeSendNotification, payload)

	err := w.HandleSendNotification(context.Background(), task)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if mockFCM.callCount != 0 {
		t.Fatalf("expected no sends for canceled subscription, got %d", mockFCM.callCount)
	}
	if repo.MarkStatusCalled {
		t.Fatal("did not expect status update for canceled subscription")
	}
}

func TestWorker_HandleSendNotification_PartialFailureDoesNotRetry(t *testing.T) {
	repo := &mockRepo{
		getSubByIDResp: &models.EventSubscription{ID: "sub-3", EntityType: EntityTypeEvent, Status: StatusPending},
		tokensResp: []*models.DeviceToken{
			{ID: "dt-1", UserID: "user-3", Token: "ok-token", Platform: "android"},
			{ID: "dt-2", UserID: "user-3", Token: "bad-token", Platform: "ios"},
		},
	}
	w := &Worker{repo: repo, fcm: nil, logger: logger.New()}
	w.sendFn = func(_ context.Context, token, _, _ string, _ map[string]string) error {
		if token == "bad-token" {
			return errInvalidToken
		}
		return nil
	}

	payload, _ := json.Marshal(SendNotificationPayload{
		UserID: "user-3", EntityID: "evt-3", EntityType: EntityTypeEvent, SubID: "sub-3",
	})
	task := newAsynqTaskForTest(TaskTypeSendNotification, payload)

	err := w.HandleSendNotification(context.Background(), task)
	if err != nil {
		t.Fatalf("expected no error on partial failure, got %v", err)
	}
	if !repo.MarkStatusCalled || repo.MarkStatusValue != StatusSent {
		t.Fatalf("expected subscription to be marked sent, got called=%v value=%q", repo.MarkStatusCalled, repo.MarkStatusValue)
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
		getSubByIDResp: &models.EventSubscription{ID: "sub-2", EntityType: EntityTypeEvent, Status: StatusPending},
		tokensResp:     []*models.DeviceToken{token},
	}

	w := &Worker{repo: repo, fcm: nil, logger: logger.New()}
	w.sendFn = mockFCM.send

	payload, _ := json.Marshal(SendNotificationPayload{
		UserID:     "user-2",
		EntityID:   "evt-2",
		EntityType: EntityTypeEvent,
		SubID:      "sub-2",
	})
	task := newAsynqTaskForTest(TaskTypeSendNotification, payload)

	err := w.HandleSendNotification(context.Background(), task)
	if err == nil {
		t.Fatal("expected error when FCM send fails, got nil")
	}

	timeout := time.After(1 * time.Second)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for !repo.DeleteTokenCalled {
		select {
		case <-ticker.C:
		case <-timeout:
			t.Fatal("expected DeleteDeviceToken to be called for invalid token")
		}
	}
}

// ===== ETA-based trigger time tests =====

func TestComputeTriggerTime_WithETA(t *testing.T) {
	mock := &mockRepo{}
	svc := newTestService(mock)
	svc.SetETAProvider(&mockETAProvider{etaSeconds: 1800}) // 30 min travel
	svc.SetLocationProvider(&mockLocationProvider{lat: 22.3, lng: 114.2})

	baseTime := time.Now().UTC().Add(3 * time.Hour)
	got := svc.computeTriggerTime(context.Background(), "user-1", baseTime, EntityTypeEvent, nil, "HKUST")

	expected := baseTime.Add(-(30*time.Minute + 10*time.Minute))
	diff := got.Sub(expected)
	if diff < -time.Second || diff > time.Second {
		t.Fatalf("expected trigger ~%v, got %v (diff %v)", expected, got, diff)
	}
}

func TestComputeTriggerTime_ETAFallbackOnError(t *testing.T) {
	mock := &mockRepo{}
	svc := newTestService(mock)
	svc.SetETAProvider(&mockETAProvider{err: errors.New("maps error")})
	svc.SetLocationProvider(&mockLocationProvider{lat: 22.3, lng: 114.2})

	baseTime := time.Now().UTC().Add(3 * time.Hour)
	got := svc.computeTriggerTime(context.Background(), "user-1", baseTime, EntityTypeEvent, nil, "HKUST")

	expected := baseTime.Add(-15 * time.Minute)
	diff := got.Sub(expected)
	if diff < -time.Second || diff > time.Second {
		t.Fatalf("expected fallback trigger ~%v, got %v", expected, got)
	}
}

func TestComputeTriggerTime_NoLocationNoETA(t *testing.T) {
	mock := &mockRepo{}
	svc := newTestService(mock)

	baseTime := time.Now().UTC().Add(3 * time.Hour)
	got := svc.computeTriggerTime(context.Background(), "user-1", baseTime, EntityTypeEvent, nil, "HKUST")

	expected := baseTime.Add(-15 * time.Minute)
	diff := got.Sub(expected)
	if diff < -time.Second || diff > time.Second {
		t.Fatalf("expected fallback trigger ~%v, got %v", expected, got)
	}
}

func TestComputeTriggerTime_TaskAlwaysFixedOffset(t *testing.T) {
	mock := &mockRepo{}
	svc := newTestService(mock)
	svc.SetETAProvider(&mockETAProvider{etaSeconds: 1800})
	svc.SetLocationProvider(&mockLocationProvider{lat: 22.3, lng: 114.2})

	baseTime := time.Now().UTC().Add(3 * time.Hour)
	got := svc.computeTriggerTime(context.Background(), "user-1", baseTime, EntityTypeTask, nil, "HKUST")

	expected := baseTime.Add(-15 * time.Minute)
	diff := got.Sub(expected)
	if diff < -time.Second || diff > time.Second {
		t.Fatalf("task should use fixed offset: expected ~%v, got %v", expected, got)
	}
}

// ===== Payload tests =====

func TestSendNotificationPayload_Backfill(t *testing.T) {
	p := SendNotificationPayload{
		UserID:  "u1",
		EventID: "evt-legacy",
		SubID:   "sub-legacy",
	}
	p.Backfill()

	if p.EntityID != "evt-legacy" {
		t.Fatalf("expected EntityID = evt-legacy, got %q", p.EntityID)
	}
	if p.EntityType != EntityTypeEvent {
		t.Fatalf("expected EntityType = event, got %q", p.EntityType)
	}
}

func TestSendNotificationPayload_BackfillNoOp(t *testing.T) {
	p := SendNotificationPayload{
		UserID:     "u1",
		EntityID:   "task-1",
		EntityType: EntityTypeTask,
		SubID:      "sub-1",
		EventID:    "should-be-ignored",
	}
	p.Backfill()

	if p.EntityID != "task-1" {
		t.Fatalf("expected EntityID unchanged, got %q", p.EntityID)
	}
	if p.EntityType != EntityTypeTask {
		t.Fatalf("expected EntityType unchanged, got %q", p.EntityType)
	}
}

func TestBuildMessage_Event(t *testing.T) {
	w := &Worker{logger: logger.New()}
	title, body, data := w.buildMessage(SendNotificationPayload{
		EntityID:   "evt-1",
		EntityType: EntityTypeEvent,
	})
	if title != "Event Reminder" {
		t.Fatalf("expected event title, got %q", title)
	}
	if body != "Your event is starting soon" {
		t.Fatalf("expected event body, got %q", body)
	}
	if data["type"] != "calendar_reminder" {
		t.Fatalf("expected calendar_reminder type, got %q", data["type"])
	}
	if data["event_id"] != "evt-1" {
		t.Fatalf("expected event_id in data, got %q", data["event_id"])
	}
}

func TestBuildMessage_Task(t *testing.T) {
	w := &Worker{logger: logger.New()}
	title, body, data := w.buildMessage(SendNotificationPayload{
		EntityID:   "task-1",
		EntityType: EntityTypeTask,
	})
	if title != "Task Reminder" {
		t.Fatalf("expected task title, got %q", title)
	}
	if body != "Your task is due soon" {
		t.Fatalf("expected task body, got %q", body)
	}
	if data["type"] != "task_reminder" {
		t.Fatalf("expected task_reminder type, got %q", data["type"])
	}
	if data["task_id"] != "task-1" {
		t.Fatalf("expected task_id in data, got %q", data["task_id"])
	}
}

// newAsynqTaskForTest creates a minimal *asynq.Task for use in handler unit tests.
func newAsynqTaskForTest(typeName string, payload []byte) *asynq.Task {
	return asynq.NewTask(typeName, payload)
}
