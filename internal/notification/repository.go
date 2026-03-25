package notification

import (
	"context"
	"time"

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
	GetPendingSubscriptions(ctx context.Context, now time.Time) ([]*models.EventSubscription, error)
	MarkSubscriptionSent(ctx context.Context, id string) error
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

// CreateSubscription inserts a new event subscription.
func (r *SQLRepository) CreateSubscription(ctx context.Context, sub *models.EventSubscription) error {
	_, err := r.pool.Pool.Exec(ctx, `
		INSERT INTO event_subscriptions (user_id, event_id, trigger_time, is_sent)
		VALUES ($1, $2, $3, false)
	`, sub.UserID, sub.EventID, sub.TriggerTime.UTC())
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
			WHERE user_id = $1 AND event_id = $2 AND is_sent = false
		)
	`, userID, eventID).Scan(&exists)
	return exists, err
}

// GetPendingSubscriptions fetches subscriptions that are due and not yet sent.
// FOR UPDATE SKIP LOCKED prevents concurrent workers from processing the same rows.
func (r *SQLRepository) GetPendingSubscriptions(ctx context.Context, now time.Time) ([]*models.EventSubscription, error) {
	rows, err := r.pool.Pool.Query(ctx, `
		SELECT id, user_id, event_id, trigger_time, is_sent, created_at
		FROM event_subscriptions
		WHERE is_sent = false AND trigger_time <= $1
		FOR UPDATE SKIP LOCKED
	`, now.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*models.EventSubscription
	for rows.Next() {
		s := &models.EventSubscription{}
		if err := rows.Scan(&s.ID, &s.UserID, &s.EventID, &s.TriggerTime, &s.IsSent, &s.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	if subs == nil {
		subs = make([]*models.EventSubscription, 0)
	}
	return subs, rows.Err()
}

// MarkSubscriptionSent sets is_sent = true for the given subscription ID.
func (r *SQLRepository) MarkSubscriptionSent(ctx context.Context, id string) error {
	_, err := r.pool.Pool.Exec(ctx, `
		UPDATE event_subscriptions SET is_sent = true WHERE id = $1
	`, id)
	return err
}

