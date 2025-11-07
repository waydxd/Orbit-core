package integration

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/waydxd/Orbit-core/internal/shared/database"
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
	db *database.DB
}

// NewSQLRepository creates a new SQL repository
func NewSQLRepository(db *database.DB) Repository {
	return &SQLRepository{db: db}
}

// CreateIntegration inserts a new integration into the database
func (r *SQLRepository) CreateIntegration(ctx context.Context, integration *models.Integration) error {
	query := `
		INSERT INTO integrations (id, user_id, service_name, api_key_encrypted, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.ExecContext(ctx, query,
		integration.ID,
		integration.UserID,
		integration.ServiceName,
		integration.APIKeyEncrypted,
		integration.Status,
		integration.CreatedAt,
		integration.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create integration: %w", err)
	}
	return nil
}

// GetIntegrationByID retrieves an integration by ID
func (r *SQLRepository) GetIntegrationByID(ctx context.Context, id string) (*models.Integration, error) {
	query := `
		SELECT id, user_id, service_name, api_key_encrypted, status, last_sync, created_at, updated_at
		FROM integrations WHERE id = $1
	`
	integration := &models.Integration{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&integration.ID,
		&integration.UserID,
		&integration.ServiceName,
		&integration.APIKeyEncrypted,
		&integration.Status,
		&integration.LastSync,
		&integration.CreatedAt,
		&integration.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("integration not found")
		}
		return nil, fmt.Errorf("failed to get integration: %w", err)
	}
	return integration, nil
}

// GetIntegrationByService retrieves an integration by user ID and service name
func (r *SQLRepository) GetIntegrationByService(ctx context.Context, userID, serviceName string) (*models.Integration, error) {
	query := `
		SELECT id, user_id, service_name, api_key_encrypted, status, last_sync, created_at, updated_at
		FROM integrations WHERE user_id = $1 AND service_name = $2
	`
	integration := &models.Integration{}
	err := r.db.QueryRowContext(ctx, query, userID, serviceName).Scan(
		&integration.ID,
		&integration.UserID,
		&integration.ServiceName,
		&integration.APIKeyEncrypted,
		&integration.Status,
		&integration.LastSync,
		&integration.CreatedAt,
		&integration.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("integration not found")
		}
		return nil, fmt.Errorf("failed to get integration: %w", err)
	}
	return integration, nil
}

// ListIntegrations retrieves all integrations for a user
func (r *SQLRepository) ListIntegrations(ctx context.Context, userID string) ([]*models.Integration, error) {
	query := `
		SELECT id, user_id, service_name, api_key_encrypted, status, last_sync, created_at, updated_at
		FROM integrations
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list integrations: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Printf("failed to close rows: %v\n", err)
		}
	}(rows)

	var integrations []*models.Integration
	for rows.Next() {
		integration := &models.Integration{}
		err := rows.Scan(
			&integration.ID,
			&integration.UserID,
			&integration.ServiceName,
			&integration.APIKeyEncrypted,
			&integration.Status,
			&integration.LastSync,
			&integration.CreatedAt,
			&integration.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan integration: %w", err)
		}
		integrations = append(integrations, integration)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating integrations: %w", err)
	}

	return integrations, nil
}

// UpdateIntegration updates an existing integration
func (r *SQLRepository) UpdateIntegration(ctx context.Context, integration *models.Integration) error {
	query := `
		UPDATE integrations
		SET service_name = $1, api_key_encrypted = $2, status = $3, updated_at = $4
		WHERE id = $5
	`
	_, err := r.db.ExecContext(ctx, query,
		integration.ServiceName,
		integration.APIKeyEncrypted,
		integration.Status,
		time.Now(),
		integration.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update integration: %w", err)
	}
	return nil
}

// DeleteIntegration deletes an integration
func (r *SQLRepository) DeleteIntegration(ctx context.Context, id string) error {
	query := "DELETE FROM integrations WHERE id = $1"
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete integration: %w", err)
	}
	return nil
}

// UpdateLastSync updates the last sync time for an integration
func (r *SQLRepository) UpdateLastSync(ctx context.Context, id string) error {
	query := `
		UPDATE integrations
		SET last_sync = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to update last sync: %w", err)
	}
	return nil
}
