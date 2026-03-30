package notification

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

const (
	// TaskTypeSendNotification is the Asynq task type for sending an FCM reminder.
	TaskTypeSendNotification = "notification:send"

	// QueueDefault is the default Asynq queue name for notification tasks.
	QueueDefault = "default"

	// QueueCritical is the high-priority Asynq queue for time-sensitive alerts.
	QueueCritical = "critical"
)

// Subscription status values stored in the database.
const (
	StatusPending   = "pending"
	StatusSent      = "sent"
	StatusCancelled = "canceled"
	StatusFailed    = "failed"
)

// SendNotificationPayload is the JSON payload stored inside an Asynq task.
type SendNotificationPayload struct {
	UserID  string `json:"user_id"`
	EventID string `json:"event_id"`
	SubID   string `json:"sub_id"`
}

// newSendNotificationTask constructs an Asynq task for the given subscription.
func newSendNotificationTask(p SendNotificationPayload) (*asynq.Task, error) {
	payload, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("newSendNotificationTask: marshal payload: %w", err)
	}
	return asynq.NewTask(TaskTypeSendNotification, payload), nil
}

// makeSendNotificationTask is overridden in tests to exercise task-construction failures.
var makeSendNotificationTask = newSendNotificationTask
