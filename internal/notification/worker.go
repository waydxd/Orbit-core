package notification

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/waydxd/Orbit-core/pkg/fcm"
	"github.com/waydxd/Orbit-core/pkg/logger"
)

// Worker wraps an Asynq server that processes send_notification tasks.
type Worker struct {
	repo   Repository
	fcm    *fcm.Client
	logger *logger.Logger
	server *asynq.Server
	// sendFn is used in tests to mock the FCM send call.
	// If nil, the real fcm.Client is used (or a no-op when fcm is nil).
	sendFn func(ctx context.Context, token, eventID, userID string, data map[string]string) error
}

// NewWorker creates a new Asynq-based notification worker.
// fcmClient may be nil when Firebase is not configured; sends will be skipped.
// server must be a configured *asynq.Server; it is started by calling Start().
func NewWorker(repo Repository, fcmClient *fcm.Client, log *logger.Logger, server *asynq.Server) *Worker {
	return &Worker{
		repo:   repo,
		fcm:    fcmClient,
		logger: log,
		server: server,
	}
}

// Start registers the task handler and launches the Asynq server in the background.
// It returns immediately; use Stop to shut it down gracefully.
func (w *Worker) Start() {
	if w.server == nil {
		w.logger.Warn("notification worker: Asynq server is nil, worker will not start")
		return
	}
	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskTypeSendNotification, w.HandleSendNotification)

	go func() {
		if err := w.server.Run(mux); err != nil {
			w.logger.Error("notification worker: Asynq server exited with error", "error", err)
		}
	}()
	w.logger.Info("notification worker started")
}

// Stop gracefully shuts down the Asynq server, waiting for in-progress tasks to finish.
func (w *Worker) Stop() {
	if w.server != nil {
		w.server.Shutdown()
		w.logger.Info("notification worker stopped")
	}
}

// HandleSendNotification is the Asynq task handler for TaskTypeSendNotification.
// It reads the payload, fetches device tokens, sends FCM messages, and updates the DB.
func (w *Worker) HandleSendNotification(ctx context.Context, t *asynq.Task) error {
	var p SendNotificationPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("HandleSendNotification: unmarshal payload: %w", err)
	}

	sub, err := w.repo.GetSubscriptionByID(ctx, p.SubID)
	if err != nil {
		return fmt.Errorf("HandleSendNotification: get subscription %s: %w", p.SubID, err)
	}
	if sub.Status != StatusPending {
		w.logger.Info("HandleSendNotification: subscription no longer pending, skipping send",
			"sub_id", p.SubID, "status", sub.Status)
		return nil
	}

	tokens, err := w.repo.GetDeviceTokensByUserID(ctx, p.UserID)
	if err != nil {
		return fmt.Errorf("HandleSendNotification: get device tokens for user %s: %w", p.UserID, err)
	}

	data := map[string]string{
		"event_id": p.EventID,
		"type":     "calendar_reminder",
		"action":   "open_event",
	}

	var sendErr error
	var successCount int
	for _, dt := range tokens {
		if err := w.sendToToken(ctx, dt.Token, p.EventID, p.UserID, data); err != nil {
			sendErr = err
			continue
		}
		successCount++
	}

	// Determine final status based on whether any send succeeded.
	if len(tokens) == 0 {
		// User has no registered device tokens; nothing to send. Log a warning but
		// still mark as sent so the task is not retried endlessly.
		w.logger.Warn("HandleSendNotification: no device tokens found for user, skipping send",
			"user_id", p.UserID, "sub_id", p.SubID)
		if err := w.repo.MarkSubscriptionStatus(ctx, p.SubID, StatusSent); err != nil {
			w.logger.Error("HandleSendNotification: failed to mark subscription sent (no tokens)",
				"sub_id", p.SubID, "error", err)
		}
		return nil
	}

	if successCount == 0 {
		if err := w.repo.MarkSubscriptionStatus(ctx, p.SubID, StatusFailed); err != nil {
			w.logger.Error("HandleSendNotification: failed to mark subscription failed",
				"sub_id", p.SubID, "error", err)
		}
		return fmt.Errorf("HandleSendNotification: all FCM sends failed: %w", sendErr)
	}

	if sendErr != nil {
		w.logger.Warn("HandleSendNotification: some device tokens failed, but at least one send succeeded",
			"sub_id", p.SubID, "user_id", p.UserID, "error", sendErr)
	}

	if err := w.repo.MarkSubscriptionStatus(ctx, p.SubID, StatusSent); err != nil {
		w.logger.Error("HandleSendNotification: failed to mark subscription sent",
			"sub_id", p.SubID, "error", err)
	}
	return nil
}

// sendToToken sends an FCM message to a single device token.
// If the token is invalid it is deleted from the database asynchronously.
func (w *Worker) sendToToken(ctx context.Context, token, eventID, userID string, data map[string]string) error {
	var err error

	if w.sendFn != nil {
		err = w.sendFn(ctx, token, eventID, userID, data)
	} else if w.fcm != nil {
		err = w.fcm.SendNotification(ctx, token, "Event Reminder", "Your event is starting soon", data)
	} else {
		w.logger.Info("notification worker: FCM client not configured, skipping send",
			"user_id", userID,
			"event_id", eventID,
		)
		return nil
	}

	if err == nil {
		w.logger.Info("notification worker: sent notification",
			"user_id", userID,
			"event_id", eventID,
			"status", "success",
		)
		return nil
	}

	w.logger.Error("notification worker: failed to send notification",
		"user_id", userID,
		"event_id", eventID,
		"status", "failure",
		"error", err,
	)

	if fcm.IsInvalidToken(err) {
		// Clean up invalid token asynchronously to unblock the main loop.
		go func(t string) {
			if delErr := w.repo.DeleteDeviceToken(context.Background(), t); delErr != nil {
				w.logger.Error("notification worker: failed to delete invalid token", "error", delErr)
			} else {
				w.logger.Info("notification worker: removed invalid token", "user_id", userID)
			}
		}(token)
	}

	return err
}
