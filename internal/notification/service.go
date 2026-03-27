package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/hibiken/asynq"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/fcm"
	"github.com/waydxd/Orbit-core/pkg/logger"
	"github.com/waydxd/Orbit-core/pkg/middleware"
)

const (
	// NotifyOffsetMinutes is how many minutes before an event's start time a reminder is sent.
	NotifyOffsetMinutes = 15
)

// TaskEnqueuer is satisfied by *asynq.Client and can be mocked in tests.
type TaskEnqueuer interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// TaskCanceller is satisfied by *asynq.Inspector and can be mocked in tests.
type TaskCanceller interface {
	// DeleteTask removes a scheduled task from the named queue.
	DeleteTask(queue, taskID string) error
}

// Service handles push-notification HTTP endpoints.
type Service struct {
	config    *config.Config
	logger    *logger.Logger
	repo      Repository
	fcm       *fcm.Client
	enqueuer  TaskEnqueuer
	canceller TaskCanceller
}

// NewService creates a new notification Service.
// fcmClient may be nil when Firebase credentials are not configured (notifications will be skipped).
// enqueuer and canceller may be nil when Redis is not available (Asynq operations will be skipped).
func NewService(cfg *config.Config, log *logger.Logger, repo Repository, fcmClient *fcm.Client, enqueuer TaskEnqueuer, canceller TaskCanceller) *Service {
	return &Service{
		config:    cfg,
		logger:    log,
		repo:      repo,
		fcm:       fcmClient,
		enqueuer:  enqueuer,
		canceller: canceller,
	}
}

// RegisterRoutes registers notification routes on the given (protected) router.
func (s *Service) RegisterRoutes(router *mux.Router) {
	// Register / unregister device token
	router.HandleFunc("/fcm/token", s.handleRegisterToken).Methods("POST")
	router.HandleFunc("/fcm/token", s.handleDeleteToken).Methods("DELETE")

	// Subscribe / unsubscribe from event notifications
	router.HandleFunc("/events/{id}/notify", s.handleSubscribe).Methods("POST")
	router.HandleFunc("/events/{id}/notify", s.handleUnsubscribe).Methods("DELETE")
}

// ===== HTTP Handlers =====

// handleRegisterToken registers (or refreshes) a device FCM token for the authenticated user.
//
//	POST /api/v1/fcm/token
//	Body: { "token": "<fcm_token>", "platform": "ios|android" }
func (s *Service) handleRegisterToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Token    string `json:"token"`
		Platform string `json:"platform"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}
	if req.Platform != "ios" && req.Platform != "android" {
		writeError(w, http.StatusBadRequest, "platform must be 'ios' or 'android'")
		return
	}

	dt := &models.DeviceToken{
		UserID:   userID,
		Token:    req.Token,
		Platform: req.Platform,
	}
	if err := s.repo.UpsertDeviceToken(r.Context(), dt); err != nil {
		s.logger.Error("failed to upsert device token", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	s.logger.Info("device token registered", "user_id", userID, "platform", req.Platform)
	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteToken removes a device FCM token for the authenticated user.
//
//	DELETE /api/v1/fcm/token
//	Body: { "token": "<fcm_token>" }
func (s *Service) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	if err := s.repo.DeleteDeviceTokenByUser(r.Context(), userID, req.Token); err != nil {
		s.logger.Error("failed to delete device token", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	s.logger.Info("device token deleted", "user_id", userID)
	w.WriteHeader(http.StatusNoContent)
}

// handleSubscribe creates an event notification subscription and enqueues an Asynq task.
//
//	POST /api/v1/events/{id}/notify
//	Body (optional): { "offset_minutes": -15 }   — defaults to -15 minutes before event start
func (s *Service) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	eventID := mux.Vars(r)["id"]
	if eventID == "" {
		writeError(w, http.StatusBadRequest, "event id is required")
		return
	}

	triggerTime, errCode, errMsg := s.validateSubscribeRequest(r.Context(), r, userID, eventID)
	if errMsg != "" {
		writeError(w, errCode, errMsg)
		return
	}

	_, errCode, errMsg = s.createAndEnqueueSubscription(r.Context(), userID, eventID, triggerTime)
	if errMsg != "" {
		writeError(w, errCode, errMsg)
		return
	}

	s.logger.Info("event subscription created",
		"user_id", userID,
		"event_id", eventID,
		"trigger_time", triggerTime.Format(time.RFC3339),
	)
	w.WriteHeader(http.StatusCreated)
}

// validateSubscribeRequest validates the subscription request and returns the trigger time.
// Returns (triggerTime, statusCode, errorMessage) where errorMessage is empty on success.
func (s *Service) validateSubscribeRequest(ctx context.Context, r *http.Request, userID, eventID string) (time.Time, int, string) {
	var req struct {
		OffsetMinutes *int       `json:"offset_minutes"`
		EventStartAt  *time.Time `json:"event_start_at"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return time.Time{}, http.StatusBadRequest, "invalid request body"
	}

	offsetMinutes := -NotifyOffsetMinutes
	if req.OffsetMinutes != nil {
		offsetMinutes = *req.OffsetMinutes
	}

	if req.EventStartAt == nil {
		return time.Time{}, http.StatusBadRequest, "event_start_at is required"
	}

	triggerTime := req.EventStartAt.UTC().Add(time.Duration(offsetMinutes) * time.Minute)
	if !triggerTime.After(time.Now().UTC()) {
		return time.Time{}, http.StatusBadRequest, "computed trigger_time is not in the future"
	}

	// Prevent duplicate subscriptions.
	exists, err := s.repo.SubscriptionExists(ctx, userID, eventID)
	if err != nil {
		s.logger.Error("failed to check subscription existence", "user_id", userID, "event_id", eventID, "error", err)
		return time.Time{}, http.StatusInternalServerError, "internal server error"
	}
	if exists {
		return time.Time{}, http.StatusConflict, "subscription already exists"
	}

	return triggerTime, http.StatusOK, ""
}

// createAndEnqueueSubscription creates a subscription and enqueues the notification task.
// Returns (subscription, statusCode, errorMessage) where errorMessage is empty on success.
func (s *Service) createAndEnqueueSubscription(ctx context.Context, userID, eventID string, triggerTime time.Time) (*models.EventSubscription, int, string) {
	sub := &models.EventSubscription{
		UserID:      userID,
		EventID:     eventID,
		TriggerTime: triggerTime,
	}

	if err := s.repo.CreateSubscription(ctx, sub); err != nil {
		if errors.Is(err, ErrSubscriptionAlreadyExists) {
			return nil, http.StatusConflict, "subscription already exists"
		}
		s.logger.Error("failed to create subscription", "user_id", userID, "event_id", eventID, "error", err)
		return nil, http.StatusInternalServerError, "internal server error"
	}

	// Enqueue the Asynq task to fire at triggerTime.
	if s.enqueuer != nil {
		task, taskErr := makeSendNotificationTask(SendNotificationPayload{
			UserID:  userID,
			EventID: eventID,
			SubID:   sub.ID,
		})
		if taskErr != nil {
			s.logger.Error("failed to construct notification task",
				"user_id", userID, "event_id", eventID, "sub_id", sub.ID, "error", taskErr)
			if delErr := s.repo.DeleteSubscription(ctx, userID, eventID); delErr != nil {
				s.logger.Error("failed to roll back subscription after task construction error",
					"user_id", userID, "event_id", eventID, "sub_id", sub.ID, "error", delErr)
			}
			return nil, http.StatusInternalServerError, "internal server error"
		}

		info, enqErr := s.enqueuer.EnqueueContext(
			ctx,
			task,
			asynq.Queue(QueueDefault),
			asynq.ProcessAt(triggerTime),
			asynq.MaxRetry(3),
		)
		if enqErr != nil {
			s.logger.Error("failed to enqueue notification task",
				"user_id", userID, "event_id", eventID, "error", enqErr)
			// Non-fatal: subscription is saved; worker can be re-implemented to poll if needed.
		} else if updErr := s.repo.UpdateSubscriptionJobID(ctx, sub.ID, info.ID); updErr != nil {
			s.logger.Error("failed to persist Asynq job_id",
				"sub_id", sub.ID, "job_id", info.ID, "error", updErr)
		}
	}

	return sub, http.StatusOK, ""
}

// handleUnsubscribe cancels the Asynq task and removes the notification subscription.
//
//	DELETE /api/v1/events/{id}/notify
func (s *Service) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	eventID := mux.Vars(r)["id"]
	if eventID == "" {
		writeError(w, http.StatusBadRequest, "event id is required")
		return
	}

	// Fetch the subscription so we can cancel the Asynq task.
	sub, err := s.repo.GetSubscriptionByUserAndEvent(r.Context(), userID, eventID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		s.logger.Error("failed to fetch subscription for cancellation",
			"user_id", userID, "event_id", eventID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Cancel the scheduled Asynq task if we have a job ID and a canceller.
	if sub != nil && sub.JobID != nil && s.canceller != nil {
		if cancelErr := s.canceller.DeleteTask(QueueDefault, *sub.JobID); cancelErr != nil {
			s.logger.Warn("failed to cancel Asynq task",
				"sub_id", sub.ID, "job_id", *sub.JobID, "error", cancelErr)
			// Non-fatal: the task may have already fired or the queue might be empty.
		}
	}

	// Mark as canceled in the database (keep row for audit).
	if sub != nil {
		if markErr := s.repo.MarkSubscriptionStatus(r.Context(), sub.ID, StatusCancelled); markErr != nil {
			s.logger.Error("failed to mark subscription canceled",
				"sub_id", sub.ID, "error", markErr)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	s.logger.Info("event subscription canceled", "user_id", userID, "event_id", eventID)
	w.WriteHeader(http.StatusNoContent)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		// Best-effort; headers already sent.
		_ = err
	}
}
