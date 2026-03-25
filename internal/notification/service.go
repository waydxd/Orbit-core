package notification

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
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

// Service handles push-notification HTTP endpoints.
type Service struct {
	config *config.Config
	logger *logger.Logger
	repo   Repository
	fcm    *fcm.Client
}

// NewService creates a new notification Service.
// fcmClient may be nil when Firebase credentials are not configured (notifications will be skipped).
func NewService(cfg *config.Config, log *logger.Logger, repo Repository, fcmClient *fcm.Client) *Service {
	return &Service{
		config: cfg,
		logger: log,
		repo:   repo,
		fcm:    fcmClient,
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

	if err := s.repo.DeleteDeviceToken(r.Context(), req.Token); err != nil {
		s.logger.Error("failed to delete device token", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	s.logger.Info("device token deleted", "user_id", userID)
	w.WriteHeader(http.StatusNoContent)
}

// handleSubscribe creates an event notification subscription for the authenticated user.
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

	var req struct {
		OffsetMinutes *int       `json:"offset_minutes"`
		EventStartAt  *time.Time `json:"event_start_at"`
	}
	// Body is optional; ignore decode error
	_ = json.NewDecoder(r.Body).Decode(&req)

	offsetMinutes := -NotifyOffsetMinutes
	if req.OffsetMinutes != nil {
		offsetMinutes = *req.OffsetMinutes
	}

	// event_start_at must be provided so we can compute trigger_time.
	if req.EventStartAt == nil {
		writeError(w, http.StatusBadRequest, "event_start_at is required")
		return
	}

	triggerTime := req.EventStartAt.UTC().Add(time.Duration(offsetMinutes) * time.Minute)
	if !triggerTime.After(time.Now().UTC()) {
		writeError(w, http.StatusBadRequest, "computed trigger_time is not in the future")
		return
	}

	// Prevent duplicate subscriptions.
	exists, err := s.repo.SubscriptionExists(r.Context(), userID, eventID)
	if err != nil {
		s.logger.Error("failed to check subscription existence", "user_id", userID, "event_id", eventID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if exists {
		writeError(w, http.StatusConflict, "subscription already exists")
		return
	}

	sub := &models.EventSubscription{
		UserID:      userID,
		EventID:     eventID,
		TriggerTime: triggerTime,
	}
	if err := s.repo.CreateSubscription(r.Context(), sub); err != nil {
		s.logger.Error("failed to create subscription", "user_id", userID, "event_id", eventID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	s.logger.Info("event subscription created",
		"user_id", userID,
		"event_id", eventID,
		"trigger_time", triggerTime.Format(time.RFC3339),
	)
	w.WriteHeader(http.StatusCreated)
}

// handleUnsubscribe removes the notification subscription for an event.
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

	if err := s.repo.DeleteSubscription(r.Context(), userID, eventID); err != nil {
		s.logger.Error("failed to delete subscription", "user_id", userID, "event_id", eventID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	s.logger.Info("event subscription deleted", "user_id", userID, "event_id", eventID)
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
