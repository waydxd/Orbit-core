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

// Entity types for notification subscriptions.
const (
	EntityTypeEvent = "event"
	EntityTypeTask  = "task"
)

// SendNotificationPayload is the JSON payload stored inside an Asynq task.
type SendNotificationPayload struct {
	UserID     string `json:"user_id"`
	EntityID   string `json:"entity_id"`
	EntityType string `json:"entity_type"`
	SubID      string `json:"sub_id"`
	// Deprecated: kept for backward compatibility with in-flight tasks enqueued
	// before the entity_type migration.
	EventID string `json:"event_id,omitempty"`
}

// Backfill sets EntityID/EntityType from the deprecated EventID field when
// the payload was produced by the previous code version.
func (p *SendNotificationPayload) Backfill() {
	if p.EntityID == "" && p.EventID != "" {
		p.EntityID = p.EventID
		p.EntityType = EntityTypeEvent
	}
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
