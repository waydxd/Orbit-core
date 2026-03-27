package notification

import (
	"context"

	"github.com/waydxd/Orbit-core/internal/shared/database"
	"github.com/waydxd/Orbit-core/internal/shared/models"
)

// Compile-time check that SQLRepository satisfies Repository.
var _ Repository = (*SQLRepository)(nil)

// Repository defines database operations for FCM notifications.
type Repository interface {
	// Device token operations
	UpsertDeviceToken(ctx context.Context, dt *models.DeviceToken) error
	DeleteDeviceToken(ctx context.Context, token string) error
	GetDeviceTokensByUserID(ctx context.Context, userID string) ([]*models.DeviceToken, error)

	// Event subscription operations
	CreateSubscription(ctx context.Context, sub *models.EventSubscription) error
	DeleteSubscription(ctx context.Context, userID, eventID string) error
	SubscriptionExists(ctx context.Context, userID, eventID string) (bool, error)
	// GetSubscriptionByID fetches a subscription by primary key.
	GetSubscriptionByID(ctx context.Context, id string) (*models.EventSubscription, error)
	// GetSubscriptionByUserAndEvent fetches an active (non-cancelled) subscription so its
	// Asynq job_id can be retrieved for task cancellation.
	GetSubscriptionByUserAndEvent(ctx context.Context, userID, eventID string) (*models.EventSubscription, error)
	// GetSubscriptionsByEventID returns all non-cancelled subscriptions for an event,
	// used when an event is rescheduled and existing tasks must be replaced.
	GetSubscriptionsByEventID(ctx context.Context, eventID string) ([]*models.EventSubscription, error)
	// MarkSubscriptionStatus updates the status field of a subscription.
	MarkSubscriptionStatus(ctx context.Context, id, status string) error
	// UpdateSubscriptionJobID stores the Asynq task ID after successful enqueue.
	UpdateSubscriptionJobID(ctx context.Context, id, jobID string) error
}

// SQLRepository implements Repository using PostgreSQL.
type SQLRepository struct {
	pool *database.DB
}

// NewSQLRepository creates a new SQL notification repository.
func NewSQLRepository(pool *database.DB) Repository {
	return &SQLRepository{pool: pool}
}

// UpsertDeviceToken inserts a new device token or updates updated_at if it already exists.
func (r *SQLRepository) UpsertDeviceToken(ctx context.Context, dt *models.DeviceToken) error {
	_, err := r.pool.Pool.Exec(ctx, `
		INSERT INTO device_tokens (user_id, token, platform, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (token) DO UPDATE
		  SET user_id    = EXCLUDED.user_id,
		      platform   = EXCLUDED.platform,
		      updated_at = NOW()
	`, dt.UserID, dt.Token, dt.Platform)
	return err
}

// DeleteDeviceToken removes a device token record by the token value.
func (r *SQLRepository) DeleteDeviceToken(ctx context.Context, token string) error {
	_, err := r.pool.Pool.Exec(ctx, `DELETE FROM device_tokens WHERE token = $1`, token)
	return err
}

// GetDeviceTokensByUserID retrieves all FCM tokens for a given user.
func (r *SQLRepository) GetDeviceTokensByUserID(ctx context.Context, userID string) ([]*models.DeviceToken, error) {
	rows, err := r.pool.Pool.Query(ctx, `
		SELECT id, user_id, token, platform, updated_at
		FROM device_tokens
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*models.DeviceToken
	for rows.Next() {
		dt := &models.DeviceToken{}
		if err := rows.Scan(&dt.ID, &dt.UserID, &dt.Token, &dt.Platform, &dt.UpdatedAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, dt)
	}
	if tokens == nil {
		tokens = make([]*models.DeviceToken, 0)
	}
	return tokens, rows.Err()
}

// CreateSubscription inserts a new event subscription with status 'pending'.
func (r *SQLRepository) CreateSubscription(ctx context.Context, sub *models.EventSubscription) error {
	err := r.pool.Pool.QueryRow(ctx, `
		INSERT INTO event_subscriptions (user_id, event_id, trigger_time, is_sent, status)
		VALUES ($1, $2, $3, false, 'pending')
		RETURNING id
	`, sub.UserID, sub.EventID, sub.TriggerTime.UTC()).Scan(&sub.ID)
	return err
}

// DeleteSubscription removes all pending subscriptions for a user + event pair.
func (r *SQLRepository) DeleteSubscription(ctx context.Context, userID, eventID string) error {
	_, err := r.pool.Pool.Exec(ctx, `
		DELETE FROM event_subscriptions
		WHERE user_id = $1 AND event_id = $2 AND is_sent = false
	`, userID, eventID)
	return err
}

// SubscriptionExists checks whether a not-yet-sent subscription already exists.
func (r *SQLRepository) SubscriptionExists(ctx context.Context, userID, eventID string) (bool, error) {
	var exists bool
	err := r.pool.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM event_subscriptions
			WHERE user_id = $1 AND event_id = $2 AND status NOT IN ('cancelled', 'sent')
		)
	`, userID, eventID).Scan(&exists)
	return exists, err
}

// GetSubscriptionByID fetches a subscription by primary key.
func (r *SQLRepository) GetSubscriptionByID(ctx context.Context, id string) (*models.EventSubscription, error) {
	sub := &models.EventSubscription{}
	err := r.pool.Pool.QueryRow(ctx, `
		SELECT id, user_id, event_id, trigger_time, is_sent, job_id, status, created_at
		FROM event_subscriptions
		WHERE id = $1
	`, id).Scan(
		&sub.ID, &sub.UserID, &sub.EventID, &sub.TriggerTime,
		&sub.IsSent, &sub.JobID, &sub.Status, &sub.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return sub, nil
}

// GetSubscriptionByUserAndEvent fetches the active subscription for a user + event pair.
func (r *SQLRepository) GetSubscriptionByUserAndEvent(ctx context.Context, userID, eventID string) (*models.EventSubscription, error) {
	sub := &models.EventSubscription{}
	err := r.pool.Pool.QueryRow(ctx, `
		SELECT id, user_id, event_id, trigger_time, is_sent, job_id, status, created_at
		FROM event_subscriptions
		WHERE user_id = $1 AND event_id = $2 AND status NOT IN ('cancelled', 'sent')
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, eventID).Scan(
		&sub.ID, &sub.UserID, &sub.EventID, &sub.TriggerTime,
		&sub.IsSent, &sub.JobID, &sub.Status, &sub.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return sub, nil
}

// GetSubscriptionsByEventID returns all active subscriptions for an event.
func (r *SQLRepository) GetSubscriptionsByEventID(ctx context.Context, eventID string) ([]*models.EventSubscription, error) {
	rows, err := r.pool.Pool.Query(ctx, `
		SELECT id, user_id, event_id, trigger_time, is_sent, job_id, status, created_at
		FROM event_subscriptions
		WHERE event_id = $1 AND status NOT IN ('cancelled', 'sent')
	`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*models.EventSubscription
	for rows.Next() {
		s := &models.EventSubscription{}
		if err := rows.Scan(&s.ID, &s.UserID, &s.EventID, &s.TriggerTime, &s.IsSent, &s.JobID, &s.Status, &s.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	if subs == nil {
		subs = make([]*models.EventSubscription, 0)
	}
	return subs, rows.Err()
}

// MarkSubscriptionStatus updates the status column for a subscription.
func (r *SQLRepository) MarkSubscriptionStatus(ctx context.Context, id, status string) error {
	isSent := status == StatusSent
	_, err := r.pool.Pool.Exec(ctx, `
		UPDATE event_subscriptions
		SET status = $2, is_sent = $3
		WHERE id = $1 AND status = 'pending'
	`, id, status, isSent)
	return err
}

// UpdateSubscriptionJobID stores the Asynq task ID for a subscription after it has been enqueued.
func (r *SQLRepository) UpdateSubscriptionJobID(ctx context.Context, id, jobID string) error {
	_, err := r.pool.Pool.Exec(ctx, `
		UPDATE event_subscriptions SET job_id = $2 WHERE id = $1
	`, id, jobID)
	return err
}
