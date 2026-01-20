package integration

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/waydxd/Orbit-core/internal/shared/database"
	"github.com/waydxd/Orbit-core/internal/shared/database/db"
	"github.com/waydxd/Orbit-core/internal/shared/models"
)

// Repository defines database operations for integrations
type Repository interface {
	CreateIntegration(ctx context.Context, integration *models.Integration) error
	GetIntegrationByID(ctx context.Context, id string) (*models.Integration, error)
	GetIntegrationByService(ctx context.Context, userID, serviceName string) (*models.Integration, error)
	ListIntegrations(ctx context.Context, userID string) ([]*models.Integration, error)
	UpdateIntegration(ctx context.Context, integration *models.Integration) error
	DeleteIntegration(ctx context.Context, id string) error
	UpdateLastSync(ctx context.Context, id string) error
}

// SQLRepository implements Repository using PostgreSQL
type SQLRepository struct {
	queries *db.Queries
	pool    *database.DB
}

// CreateIntegration inserts a new integration into the database
func (r *SQLRepository) CreateIntegration(ctx context.Context, integration *models.Integration) error {
	params := db.CreateIntegrationParams{
		ID:              database.StringToUUID(integration.ID),
		UserID:          database.StringToUUID(integration.UserID),
		ServiceName:     integration.ServiceName,
		ApiKeyEncrypted: integration.APIKeyEncrypted,
		Status:          pgtype.Text{String: integration.Status, Valid: integration.Status != ""},
		CreatedAt:       database.TimeToTimestamptz(integration.CreatedAt),
		UpdatedAt:       database.TimeToTimestamptz(integration.UpdatedAt),
	}

	err := r.queries.CreateIntegration(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to create integration: %w", err)
	}
	return nil
}

// GetIntegrationByID retrieves an integration by ID
func (r *SQLRepository) GetIntegrationByID(ctx context.Context, id string) (*models.Integration, error) {
	row, err := r.queries.GetIntegrationByID(ctx, database.StringToUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("integration not found")
		}
		return nil, fmt.Errorf("failed to get integration: %w", err)
	}

	return &models.Integration{
		ID:              database.UUIDToString(row.ID),
		UserID:          database.UUIDToString(row.UserID),
		ServiceName:     row.ServiceName,
		APIKeyEncrypted: row.ApiKeyEncrypted,
		Status:          database.TextToString(row.Status),
		LastSync:        database.TimestamptzToTime(row.LastSync),
		CreatedAt:       database.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:       database.TimestamptzToTime(row.UpdatedAt),
	}, nil
}

// GetIntegrationByService retrieves an integration by user ID and service name
func (r *SQLRepository) GetIntegrationByService(ctx context.Context, userID, serviceName string) (*models.Integration, error) {
	params := db.GetIntegrationByServiceParams{
		UserID:      database.StringToUUID(userID),
		ServiceName: serviceName,
	}
	row, err := r.queries.GetIntegrationByService(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("integration not found")
		}
		return nil, fmt.Errorf("failed to get integration: %w", err)
	}

	return &models.Integration{
		ID:              database.UUIDToString(row.ID),
		UserID:          database.UUIDToString(row.UserID),
		ServiceName:     row.ServiceName,
		APIKeyEncrypted: row.ApiKeyEncrypted,
		Status:          database.TextToString(row.Status),
		LastSync:        database.TimestamptzToTime(row.LastSync),
		CreatedAt:       database.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:       database.TimestamptzToTime(row.UpdatedAt),
	}, nil
}

// ListIntegrations retrieves all integrations for a user
func (r *SQLRepository) ListIntegrations(ctx context.Context, userID string) ([]*models.Integration, error) {
	rows, err := r.queries.ListIntegrations(ctx, database.StringToUUID(userID))
	if err != nil {
		return nil, fmt.Errorf("failed to list integrations: %w", err)
	}

	var integrations []*models.Integration
	for _, row := range rows {
		integrations = append(integrations, &models.Integration{
			ID:              database.UUIDToString(row.ID),
			UserID:          database.UUIDToString(row.UserID),
			ServiceName:     row.ServiceName,
			APIKeyEncrypted: row.ApiKeyEncrypted,
			Status:          database.TextToString(row.Status),
			LastSync:        database.TimestamptzToTime(row.LastSync),
			CreatedAt:       database.TimestamptzToTime(row.CreatedAt),
			UpdatedAt:       database.TimestamptzToTime(row.UpdatedAt),
		})
	}

	return integrations, nil
}

// UpdateIntegration updates an existing integration
func (r *SQLRepository) UpdateIntegration(ctx context.Context, integration *models.Integration) error {
	params := db.UpdateIntegrationParams{
		ServiceName:     integration.ServiceName,
		ApiKeyEncrypted: integration.APIKeyEncrypted,
		Status:          pgtype.Text{String: integration.Status, Valid: integration.Status != ""},
		UpdatedAt:       database.TimeToTimestamptz(time.Now()),
		ID:              database.StringToUUID(integration.ID),
	}

	err := r.queries.UpdateIntegration(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to update integration: %w", err)
	}
	return nil
}

// DeleteIntegration deletes an integration
func (r *SQLRepository) DeleteIntegration(ctx context.Context, id string) error {
	err := r.queries.DeleteIntegration(ctx, database.StringToUUID(id))
	if err != nil {
		return fmt.Errorf("failed to delete integration: %w", err)
	}
	return nil
}

// UpdateLastSync updates the last sync time for an integration
func (r *SQLRepository) UpdateLastSync(ctx context.Context, id string) error {
	err := r.queries.UpdateLastSync(ctx, database.StringToUUID(id))
	if err != nil {
		return fmt.Errorf("failed to update last sync: %w", err)
	}
	return nil
}
