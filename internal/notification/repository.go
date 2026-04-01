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
	DeleteDeviceTokenByUser(ctx context.Context, userID, token string) error
	GetDeviceTokensByUserID(ctx context.Context, userID string) ([]*models.DeviceToken, error)

	// Subscription operations (entity_type distinguishes events from tasks)
	CreateSubscription(ctx context.Context, sub *models.EventSubscription) error
	DeleteSubscription(ctx context.Context, userID, entityID, entityType string) error
	SubscriptionExists(ctx context.Context, userID, entityID, entityType string) (bool, error)
	GetSubscriptionByID(ctx context.Context, id string) (*models.EventSubscription, error)
	GetSubscriptionByUserAndEntity(ctx context.Context, userID, entityID, entityType string) (*models.EventSubscription, error)
	GetSubscriptionsByEntityID(ctx context.Context, entityID, entityType string) ([]*models.EventSubscription, error)
	MarkSubscriptionStatus(ctx context.Context, id, status string) error
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

// CreateSubscription inserts a new subscription with status 'pending'.
func (r *SQLRepository) CreateSubscription(ctx context.Context, sub *models.EventSubscription) error {
	params := db.CreateSubscriptionParams{
		UserID:      database.StringToUUID(sub.UserID),
		EntityID:    database.StringToUUID(sub.EntityID),
		EntityType:  sub.EntityType,
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

// DeleteSubscription removes all pending subscriptions for a user + entity pair.
func (r *SQLRepository) DeleteSubscription(ctx context.Context, userID, entityID, entityType string) error {
	params := db.DeleteSubscriptionParams{
		UserID:     database.StringToUUID(userID),
		EntityID:   database.StringToUUID(entityID),
		EntityType: entityType,
	}
	return r.queries.DeleteSubscription(ctx, params)
}

// DeleteDeviceTokenByUser removes a device token record scoped to a specific user.
func (r *SQLRepository) DeleteDeviceTokenByUser(ctx context.Context, userID, token string) error {
	params := db.DeleteDeviceTokenByUserParams{UserID: database.StringToUUID(userID), Token: token}
	return r.queries.DeleteDeviceTokenByUser(ctx, params)
}

// SubscriptionExists checks whether a not-yet-sent subscription already exists.
func (r *SQLRepository) SubscriptionExists(ctx context.Context, userID, entityID, entityType string) (bool, error) {
	params := db.SubscriptionExistsParams{
		UserID:     database.StringToUUID(userID),
		EntityID:   database.StringToUUID(entityID),
		EntityType: entityType,
	}
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
		EntityID:    database.UUIDToString(row.EntityID),
		EntityType:  row.EntityType,
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

// GetSubscriptionByUserAndEntity fetches the active subscription for a user + entity pair.
func (r *SQLRepository) GetSubscriptionByUserAndEntity(ctx context.Context, userID, entityID, entityType string) (*models.EventSubscription, error) {
	params := db.GetSubscriptionByUserAndEntityParams{
		UserID:     database.StringToUUID(userID),
		EntityID:   database.StringToUUID(entityID),
		EntityType: entityType,
	}
	row, err := r.queries.GetSubscriptionByUserAndEntity(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	sub := &models.EventSubscription{
		ID:          database.UUIDToString(row.ID),
		UserID:      database.UUIDToString(row.UserID),
		EntityID:    database.UUIDToString(row.EntityID),
		EntityType:  row.EntityType,
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

// GetSubscriptionsByEntityID returns all active subscriptions for an entity.
func (r *SQLRepository) GetSubscriptionsByEntityID(ctx context.Context, entityID, entityType string) ([]*models.EventSubscription, error) {
	params := db.GetSubscriptionsByEntityIDParams{
		EntityID:   database.StringToUUID(entityID),
		EntityType: entityType,
	}
	rows, err := r.queries.GetSubscriptionsByEntityID(ctx, params)
	if err != nil {
		return nil, err
	}
	var subs []*models.EventSubscription
	for _, row := range rows {
		s := &models.EventSubscription{
			ID:          database.UUIDToString(row.ID),
			UserID:      database.UUIDToString(row.UserID),
			EntityID:    database.UUIDToString(row.EntityID),
			EntityType:  row.EntityType,
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
