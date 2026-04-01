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
	// NotifyOffsetMinutes is the default offset before an entity's start/due time.
	NotifyOffsetMinutes = 15
)

// ETAProvider computes travel time (seconds) from the user's current location
// to a destination address. Implementations may call the Google Distance Matrix
// API or return an error to signal a fallback.
type ETAProvider interface {
	GetETA(ctx context.Context, originLat, originLng float64, destinationAddress string) (int, error)
}

// LocationProvider returns the user's latest known location.
type LocationProvider interface {
	GetCurrentLocation(ctx context.Context, userID string) (lat, lng float64, err error)
}

// TaskEnqueuer is satisfied by *asynq.Client and can be mocked in tests.
type TaskEnqueuer interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// TaskCanceller is satisfied by *asynq.Inspector and can be mocked in tests.
type TaskCanceller interface {
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
	eta       ETAProvider
	location  LocationProvider
}

// NewService creates a new notification Service.
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

// SetETAProvider sets the ETA provider used for location-aware event reminders.
func (s *Service) SetETAProvider(eta ETAProvider) { s.eta = eta }

// SetLocationProvider sets the location provider used to find the user's origin.
func (s *Service) SetLocationProvider(loc LocationProvider) { s.location = loc }

// RegisterRoutes registers notification routes on the given (protected) router.
func (s *Service) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/fcm/token", s.handleRegisterToken).Methods("POST")
	router.HandleFunc("/fcm/token", s.handleDeleteToken).Methods("DELETE")

	router.HandleFunc("/events/{id}/notify", s.handleSubscribeEvent).Methods("POST")
	router.HandleFunc("/events/{id}/notify", s.handleUnsubscribeEvent).Methods("DELETE")

	router.HandleFunc("/tasks/{id}/notify", s.handleSubscribeTask).Methods("POST")
	router.HandleFunc("/tasks/{id}/notify", s.handleUnsubscribeTask).Methods("DELETE")
}

// ===== HTTP Handlers =====

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

// ===== Event subscribe / unsubscribe =====

func (s *Service) handleSubscribeEvent(w http.ResponseWriter, r *http.Request) {
	s.handleSubscribe(w, r, EntityTypeEvent)
}

func (s *Service) handleUnsubscribeEvent(w http.ResponseWriter, r *http.Request) {
	s.handleUnsubscribe(w, r, EntityTypeEvent)
}

// ===== Task subscribe / unsubscribe =====

func (s *Service) handleSubscribeTask(w http.ResponseWriter, r *http.Request) {
	s.handleSubscribe(w, r, EntityTypeTask)
}

func (s *Service) handleUnsubscribeTask(w http.ResponseWriter, r *http.Request) {
	s.handleUnsubscribe(w, r, EntityTypeTask)
}

// ===== Generic subscribe/unsubscribe logic =====

func (s *Service) handleSubscribe(w http.ResponseWriter, r *http.Request, entityType string) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	entityID := mux.Vars(r)["id"]
	if entityID == "" {
		writeError(w, http.StatusBadRequest, "entity id is required")
		return
	}

	triggerTime, errCode, errMsg := s.validateSubscribeRequest(r.Context(), r, userID, entityID, entityType)
	if errMsg != "" {
		writeError(w, errCode, errMsg)
		return
	}

	_, errCode, errMsg = s.createAndEnqueueSubscription(r.Context(), userID, entityID, entityType, triggerTime)
	if errMsg != "" {
		writeError(w, errCode, errMsg)
		return
	}

	s.logger.Info("subscription created",
		"user_id", userID,
		"entity_id", entityID,
		"entity_type", entityType,
		"trigger_time", triggerTime.Format(time.RFC3339),
	)
	w.WriteHeader(http.StatusCreated)
}

func (s *Service) validateSubscribeRequest(ctx context.Context, r *http.Request, userID, entityID, entityType string) (time.Time, int, string) {
	var req struct {
		OffsetMinutes *int       `json:"offset_minutes"`
		EventStartAt  *time.Time `json:"event_start_at"`
		TaskDueAt     *time.Time `json:"task_due_at"`
		Location      string     `json:"location"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return time.Time{}, http.StatusBadRequest, "invalid request body"
	}

	var baseTime *time.Time
	switch entityType {
	case EntityTypeEvent:
		baseTime = req.EventStartAt
	case EntityTypeTask:
		baseTime = req.TaskDueAt
	}
	if baseTime == nil {
		label := "event_start_at"
		if entityType == EntityTypeTask {
			label = "task_due_at"
		}
		return time.Time{}, http.StatusBadRequest, label + " is required"
	}

	triggerTime := s.computeTriggerTime(ctx, userID, *baseTime, entityType, req.OffsetMinutes, req.Location)

	if !triggerTime.After(time.Now().UTC()) {
		return time.Time{}, http.StatusBadRequest, "computed trigger_time is not in the future"
	}

	exists, err := s.repo.SubscriptionExists(ctx, userID, entityID, entityType)
	if err != nil {
		s.logger.Error("failed to check subscription existence", "user_id", userID, "entity_id", entityID, "error", err)
		return time.Time{}, http.StatusInternalServerError, "internal server error"
	}
	if exists {
		return time.Time{}, http.StatusConflict, "subscription already exists"
	}

	return triggerTime, http.StatusOK, ""
}

// computeTriggerTime determines the notification fire time.
// For events: tries ETA-based scheduling (ETA + 10min buffer), falls back to fixed offset.
// For tasks: always uses fixed offset.
func (s *Service) computeTriggerTime(ctx context.Context, userID string, baseTime time.Time, entityType string, offsetMinutes *int, locationHint string) time.Time {
	if entityType == EntityTypeEvent && locationHint != "" && s.eta != nil && s.location != nil {
		lat, lng, err := s.location.GetCurrentLocation(ctx, userID)
		if err == nil {
			etaCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			etaSeconds, etaErr := s.eta.GetETA(etaCtx, lat, lng, locationHint)
			if etaErr == nil && etaSeconds > 0 {
				etaDuration := time.Duration(etaSeconds)*time.Second + 10*time.Minute
				return baseTime.UTC().Add(-etaDuration)
			}
		}
	}

	offset := -NotifyOffsetMinutes
	if offsetMinutes != nil {
		offset = *offsetMinutes
	}
	return baseTime.UTC().Add(time.Duration(offset) * time.Minute)
}

func (s *Service) createAndEnqueueSubscription(ctx context.Context, userID, entityID, entityType string, triggerTime time.Time) (*models.EventSubscription, int, string) {
	sub := &models.EventSubscription{
		UserID:      userID,
		EntityID:    entityID,
		EntityType:  entityType,
		TriggerTime: triggerTime,
	}

	if err := s.repo.CreateSubscription(ctx, sub); err != nil {
		if errors.Is(err, ErrSubscriptionAlreadyExists) {
			return nil, http.StatusConflict, "subscription already exists"
		}
		s.logger.Error("failed to create subscription", "user_id", userID, "entity_id", entityID, "error", err)
		return nil, http.StatusInternalServerError, "internal server error"
	}

	if s.enqueuer != nil {
		task, taskErr := makeSendNotificationTask(SendNotificationPayload{
			UserID:     userID,
			EntityID:   entityID,
			EntityType: entityType,
			SubID:      sub.ID,
		})
		if taskErr != nil {
			s.logger.Error("failed to construct notification task",
				"user_id", userID, "entity_id", entityID, "sub_id", sub.ID, "error", taskErr)
			if delErr := s.repo.DeleteSubscription(ctx, userID, entityID, entityType); delErr != nil {
				s.logger.Error("failed to roll back subscription after task construction error",
					"user_id", userID, "entity_id", entityID, "sub_id", sub.ID, "error", delErr)
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
				"user_id", userID, "entity_id", entityID, "error", enqErr)
		} else if updErr := s.repo.UpdateSubscriptionJobID(ctx, sub.ID, info.ID); updErr != nil {
			s.logger.Error("failed to persist Asynq job_id",
				"sub_id", sub.ID, "job_id", info.ID, "error", updErr)
		}
	}

	return sub, http.StatusOK, ""
}

func (s *Service) handleUnsubscribe(w http.ResponseWriter, r *http.Request, entityType string) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	entityID := mux.Vars(r)["id"]
	if entityID == "" {
		writeError(w, http.StatusBadRequest, "entity id is required")
		return
	}

	sub, err := s.repo.GetSubscriptionByUserAndEntity(r.Context(), userID, entityID, entityType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		s.logger.Error("failed to fetch subscription for cancellation",
			"user_id", userID, "entity_id", entityID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if sub != nil && sub.JobID != nil && s.canceller != nil {
		if cancelErr := s.canceller.DeleteTask(QueueDefault, *sub.JobID); cancelErr != nil {
			s.logger.Warn("failed to cancel Asynq task",
				"sub_id", sub.ID, "job_id", *sub.JobID, "error", cancelErr)
		}
	}

	if sub != nil {
		if markErr := s.repo.MarkSubscriptionStatus(r.Context(), sub.ID, StatusCancelled); markErr != nil {
			s.logger.Error("failed to mark subscription canceled",
				"sub_id", sub.ID, "error", markErr)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	s.logger.Info("subscription canceled", "user_id", userID, "entity_id", entityID, "entity_type", entityType)
	w.WriteHeader(http.StatusNoContent)
}

func writeError(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		_ = err
	}
}
