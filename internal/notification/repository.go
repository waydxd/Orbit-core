package notification

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/waydxd/Orbit-core/internal/shared/database"
	"github.com/waydxd/Orbit-core/internal/shared/database/db"
	"github.com/waydxd/Orbit-core/internal/shared/models"
)

// Compile-time check that SQLRepository satisfies Repository.
var _ Repository = (*SQLRepository)(nil)

// ErrSubscriptionAlreadyExists is returned when an attempt to create a subscription
// conflicts with an existing active subscription.
var ErrSubscriptionAlreadyExists = errors.New("subscription already exists")

// Repository defines database operations for FCM notifications.
type Repository interface {
	// Device token operations
	UpsertDeviceToken(ctx context.Context, dt *models.DeviceToken) error
	DeleteDeviceToken(ctx context.Context, token string) error
	// DeleteDeviceTokenByUser removes a device token for a specific user.
	DeleteDeviceTokenByUser(ctx context.Context, userID, token string) error
	GetDeviceTokensByUserID(ctx context.Context, userID string) ([]*models.DeviceToken, error)

	// Event subscription operations
	CreateSubscription(ctx context.Context, sub *models.EventSubscription) error
	DeleteSubscription(ctx context.Context, userID, eventID string) error
	SubscriptionExists(ctx context.Context, userID, eventID string) (bool, error)
	// GetSubscriptionByID fetches a subscription by primary key.
	GetSubscriptionByID(ctx context.Context, id string) (*models.EventSubscription, error)
	// GetSubscriptionByUserAndEvent fetches an active (non-canceled) subscription so its
	// Asynq job_id can be retrieved for task cancellation.
	GetSubscriptionByUserAndEvent(ctx context.Context, userID, eventID string) (*models.EventSubscription, error)
	// GetSubscriptionsByEventID returns all non-canceled subscriptions for an event,
	// used when an event is rescheduled and existing tasks must be replaced.
	GetSubscriptionsByEventID(ctx context.Context, eventID string) ([]*models.EventSubscription, error)
	// MarkSubscriptionStatus updates the status field of a subscription.
	MarkSubscriptionStatus(ctx context.Context, id, status string) error
	// UpdateSubscriptionJobID stores the Asynq task ID after successful enqueue.
	UpdateSubscriptionJobID(ctx context.Context, id, jobID string) error
}

// SQLRepository implements Repository using PostgreSQL.
type SQLRepository struct {
	queries *db.Queries
	pool    *database.DB
}

// NewSQLRepository creates a new SQL notification repository.
func NewSQLRepository(pool *database.DB) Repository {
	return &SQLRepository{queries: db.New(pool.Pool), pool: pool}
}

// UpsertDeviceToken inserts a new device token or updates updated_at if it already exists.
func (r *SQLRepository) UpsertDeviceToken(ctx context.Context, dt *models.DeviceToken) error {
	params := db.UpsertDeviceTokenParams{
		UserID:   database.StringToUUID(dt.UserID),
		Token:    dt.Token,
		Platform: dt.Platform,
	}
	return r.queries.UpsertDeviceToken(ctx, params)
}

// DeleteDeviceToken removes a device token record by the token value.
func (r *SQLRepository) DeleteDeviceToken(ctx context.Context, token string) error {
	return r.queries.DeleteDeviceToken(ctx, token)
}

// GetDeviceTokensByUserID retrieves all FCM tokens for a given user.
func (r *SQLRepository) GetDeviceTokensByUserID(ctx context.Context, userID string) ([]*models.DeviceToken, error) {
	rows, err := r.queries.GetDeviceTokensByUserID(ctx, database.StringToUUID(userID))
	if err != nil {
		return nil, err
	}
	var tokens []*models.DeviceToken
	for _, row := range rows {
		dt := &models.DeviceToken{
			ID:        database.UUIDToString(row.ID),
			UserID:    database.UUIDToString(row.UserID),
			Token:     row.Token,
			Platform:  row.Platform,
			UpdatedAt: database.TimestamptzToTime(row.UpdatedAt),
		}
		tokens = append(tokens, dt)
	}
	if tokens == nil {
		tokens = make([]*models.DeviceToken, 0)
	}
	return tokens, nil
}

// CreateSubscription inserts a new event subscription with status 'pending'.
func (r *SQLRepository) CreateSubscription(ctx context.Context, sub *models.EventSubscription) error {
	params := db.CreateSubscriptionParams{
		UserID:      database.StringToUUID(sub.UserID),
		EventID:     database.StringToUUID(sub.EventID),
		TriggerTime: database.TimeToTimestamptz(sub.TriggerTime),
	}
	id, err := r.queries.CreateSubscription(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSubscriptionAlreadyExists
		}
		return err
	}
	sub.ID = database.UUIDToString(id)
	return nil
}

// DeleteSubscription removes all pending subscriptions for a user + event pair.
func (r *SQLRepository) DeleteSubscription(ctx context.Context, userID, eventID string) error {
	params := db.DeleteSubscriptionParams{UserID: database.StringToUUID(userID), EventID: database.StringToUUID(eventID)}
	return r.queries.DeleteSubscription(ctx, params)
}

// DeleteDeviceTokenByUser removes a device token record scoped to a specific user.
func (r *SQLRepository) DeleteDeviceTokenByUser(ctx context.Context, userID, token string) error {
	params := db.DeleteDeviceTokenByUserParams{UserID: database.StringToUUID(userID), Token: token}
	return r.queries.DeleteDeviceTokenByUser(ctx, params)
}

// SubscriptionExists checks whether a not-yet-sent subscription already exists.
func (r *SQLRepository) SubscriptionExists(ctx context.Context, userID, eventID string) (bool, error) {
	params := db.SubscriptionExistsParams{UserID: database.StringToUUID(userID), EventID: database.StringToUUID(eventID)}
	return r.queries.SubscriptionExists(ctx, params)
}

// GetSubscriptionByID fetches a subscription by primary key.
func (r *SQLRepository) GetSubscriptionByID(ctx context.Context, id string) (*models.EventSubscription, error) {
	row, err := r.queries.GetSubscriptionByID(ctx, database.StringToUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	sub := &models.EventSubscription{
		ID:          database.UUIDToString(row.ID),
		UserID:      database.UUIDToString(row.UserID),
		EventID:     database.UUIDToString(row.EventID),
		TriggerTime: database.TimestamptzToTime(row.TriggerTime),
		IsSent:      row.IsSent,
		Status:      row.Status,
		CreatedAt:   database.TimestamptzToTime(row.CreatedAt),
	}
	if row.JobID.Valid {
		v := row.JobID.String
		sub.JobID = &v
	}
	return sub, nil
}

// GetSubscriptionByUserAndEvent fetches the active subscription for a user + event pair.
func (r *SQLRepository) GetSubscriptionByUserAndEvent(ctx context.Context, userID, eventID string) (*models.EventSubscription, error) {
	params := db.GetSubscriptionByUserAndEventParams{UserID: database.StringToUUID(userID), EventID: database.StringToUUID(eventID)}
	row, err := r.queries.GetSubscriptionByUserAndEvent(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	sub := &models.EventSubscription{
		ID:          database.UUIDToString(row.ID),
		UserID:      database.UUIDToString(row.UserID),
		EventID:     database.UUIDToString(row.EventID),
		TriggerTime: database.TimestamptzToTime(row.TriggerTime),
		IsSent:      row.IsSent,
		Status:      row.Status,
		CreatedAt:   database.TimestamptzToTime(row.CreatedAt),
	}
	if row.JobID.Valid {
		v := row.JobID.String
		sub.JobID = &v
	}
	return sub, nil
}

// GetSubscriptionsByEventID returns all active subscriptions for an event.
func (r *SQLRepository) GetSubscriptionsByEventID(ctx context.Context, eventID string) ([]*models.EventSubscription, error) {
	rows, err := r.queries.GetSubscriptionsByEventID(ctx, database.StringToUUID(eventID))
	if err != nil {
		return nil, err
	}
	var subs []*models.EventSubscription
	for _, row := range rows {
		s := &models.EventSubscription{
			ID:          database.UUIDToString(row.ID),
			UserID:      database.UUIDToString(row.UserID),
			EventID:     database.UUIDToString(row.EventID),
			TriggerTime: database.TimestamptzToTime(row.TriggerTime),
			IsSent:      row.IsSent,
			Status:      row.Status,
			CreatedAt:   database.TimestamptzToTime(row.CreatedAt),
		}
		if row.JobID.Valid {
			v := row.JobID.String
			s.JobID = &v
		}
		subs = append(subs, s)
	}
	if subs == nil {
		subs = make([]*models.EventSubscription, 0)
	}
	return subs, nil
}

// MarkSubscriptionStatus updates the status column for a subscription.
func (r *SQLRepository) MarkSubscriptionStatus(ctx context.Context, id, status string) error {
	isSent := status == StatusSent
	params := db.MarkSubscriptionStatusParams{ID: database.StringToUUID(id), Status: status, IsSent: isSent}
	return r.queries.MarkSubscriptionStatus(ctx, params)
}

// UpdateSubscriptionJobID stores the Asynq task ID for a subscription after it has been enqueued.
func (r *SQLRepository) UpdateSubscriptionJobID(ctx context.Context, id, jobID string) error {
	params := db.UpdateSubscriptionJobIDParams{ID: database.StringToUUID(id), JobID: pgtype.Text{String: jobID, Valid: jobID != ""}}
	return r.queries.UpdateSubscriptionJobID(ctx, params)
}
