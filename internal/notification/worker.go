package notification

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/fcm"
	"github.com/waydxd/Orbit-core/pkg/logger"
)

// Worker polls the database every minute and dispatches pending FCM notifications.
type Worker struct {
	repo   Repository
	fcm    *fcm.Client
	logger *logger.Logger
	c      *cron.Cron
	// sendFn is used in tests to mock the FCM send call.
	// If nil, the real fcm.Client is used (or a no-op when fcm is nil).
	sendFn func(ctx context.Context, token, eventID, userID string, data map[string]string) error
}

// NewWorker creates a new notification worker.
// fcmClient may be nil when Firebase is not configured; sends will be skipped.
func NewWorker(repo Repository, fcmClient *fcm.Client, log *logger.Logger) *Worker {
	return &Worker{
		repo:   repo,
		fcm:    fcmClient,
		logger: log,
	}
}

// Start launches the cron scheduler in the background.
// It returns immediately; use Stop to shut it down gracefully.
func (w *Worker) Start() {
	w.c = cron.New()
	_, err := w.c.AddFunc("* * * * *", func() { // every minute
		w.run(context.Background())
	})
	if err != nil {
		w.logger.Error("notification worker: failed to add cron job", "error", err)
		return
	}
	w.c.Start()
	w.logger.Info("notification worker started")
}

// Stop gracefully stops the cron scheduler and waits for any in-progress run to finish.
func (w *Worker) Stop() {
	if w.c != nil {
		ctx := w.c.Stop()
		<-ctx.Done()
		w.logger.Info("notification worker stopped")
	}
}

// run is the body of the cron job: fetches pending subscriptions, sends FCM messages,
// and marks them as sent.
func (w *Worker) run(ctx context.Context) {
	now := time.Now().UTC()

	subs, err := w.repo.GetPendingSubscriptions(ctx, now)
	if err != nil {
		w.logger.Error("notification worker: failed to fetch pending subscriptions", "error", err)
		return
	}

	for _, sub := range subs {
		w.process(ctx, sub)
	}
}

// process sends notifications for a single subscription and updates the database.
func (w *Worker) process(ctx context.Context, sub *models.EventSubscription) {
	// Fetch device tokens for the user
	tokens, err := w.repo.GetDeviceTokensByUserID(ctx, sub.UserID)
	if err != nil {
		w.logger.Error("notification worker: failed to get device tokens",
			"user_id", sub.UserID,
			"subscription_id", sub.ID,
			"error", err,
		)
		return
	}

	data := map[string]string{
		"event_id": sub.EventID,
		"type":     "calendar_reminder",
		"action":   "open_event",
	}

	for _, dt := range tokens {
		w.sendToToken(ctx, dt.Token, sub.EventID, sub.UserID, data)
	}

	// Always mark the subscription as sent after processing all tokens.
	// This ensures idempotency: the worker won't retry indefinitely if all sends
	// fail (e.g. transient errors). Failed sends are logged for observability.
	if err := w.repo.MarkSubscriptionSent(ctx, sub.ID); err != nil {
		w.logger.Error("notification worker: failed to mark subscription sent",
			"subscription_id", sub.ID,
			"error", err,
		)
	}
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
