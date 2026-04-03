package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
	sendFn func(ctx context.Context, token, entityID, userID string, data map[string]string) error
}

const invalidTokenDeleteTimeout = 2 * time.Second

// NewWorker creates a new notification worker.
func NewWorker(repo Repository, fcmClient *fcm.Client, log *logger.Logger, server *asynq.Server) *Worker {
	return &Worker{
		repo:   repo,
		fcm:    fcmClient,
		logger: log,
		server: server,
	}
}

// Start registers the task handler and launches the Asynq server in the background.
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

// Stop gracefully shuts down the Asynq server.
func (w *Worker) Stop() {
	if w.server != nil {
		w.server.Shutdown()
		w.logger.Info("notification worker stopped")
	}
}

// HandleSendNotification is the Asynq task handler for TaskTypeSendNotification.
func (w *Worker) HandleSendNotification(ctx context.Context, t *asynq.Task) error {
	var p SendNotificationPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("HandleSendNotification: unmarshal payload: %w", err)
	}
	p.Backfill()

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

	title, body, data := w.buildMessage(p)

	if len(tokens) == 0 {
		w.logger.Warn("HandleSendNotification: no device tokens found for user, skipping send",
			"user_id", p.UserID, "sub_id", p.SubID)
		if err := w.repo.MarkSubscriptionStatus(ctx, p.SubID, StatusSent); err != nil {
			w.logger.Error("HandleSendNotification: failed to mark subscription sent (no tokens)",
				"sub_id", p.SubID, "error", err)
		}
		return nil
	}

	var sendErr error
	var successCount int
	for _, dt := range tokens {
		if err := w.sendToToken(ctx, dt.Token, p.EntityID, p.UserID, title, body, data); err != nil {
			sendErr = err
			continue
		}
		successCount++
	}

	if successCount == 0 {
		if err := w.repo.MarkSubscriptionStatus(ctx, p.SubID, StatusFailed); err != nil {
			w.logger.Error("HandleSendNotification: failed to mark subscription failed",
				"sub_id", p.SubID, "error", err)
		}
		w.logger.Error("HandleSendNotification: all FCM sends failed; subscription marked as failed, retrying",
			"sub_id", p.SubID, "user_id", p.UserID, "error", sendErr)
		return sendErr
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

// buildMessage produces notification title, body and FCM data payload based on entity type.
func (w *Worker) buildMessage(p SendNotificationPayload) (string, string, map[string]string) {
	switch p.EntityType {
	case EntityTypeTask:
		return "Task Reminder", "Your task is due soon", map[string]string{
			"task_id": p.EntityID,
			"type":    "task_reminder",
			"action":  "open_task",
		}
	default:
		return "Event Reminder", "Your event is starting soon", map[string]string{
			"event_id": p.EntityID,
			"type":     "calendar_reminder",
			"action":   "open_event",
		}
	}
}

func (w *Worker) sendToToken(ctx context.Context, token, entityID, userID, title, body string, data map[string]string) error {
	var err error
	switch {
	case w.sendFn != nil:
		err = w.sendFn(ctx, token, entityID, userID, data)
	case w.fcm != nil:
		err = w.fcm.SendNotification(ctx, token, title, body, data)
	default:
		w.logger.Info("notification worker: FCM client not configured, skipping send",
			"user_id", userID,
			"entity_id", entityID,
		)
		return nil
	}

	if err == nil {
		w.logger.Info("notification worker: sent notification",
			"user_id", userID,
			"entity_id", entityID,
			"status", "success",
		)
		return nil
	}

	w.logger.Error("notification worker: failed to send notification",
		"user_id", userID,
		"entity_id", entityID,
		"status", "failure",
		"error", err,
	)

	if fcm.IsInvalidToken(err) {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), invalidTokenDeleteTimeout)
		defer cancel()

		if delErr := w.repo.DeleteDeviceToken(cleanupCtx, token); delErr != nil {
			w.logger.Error("notification worker: failed to delete invalid token", "error", delErr)
		} else {
			w.logger.Info("notification worker: removed invalid token", "user_id", userID)
		}
	}

	return err
}
