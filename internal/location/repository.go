package location

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/waydxd/Orbit-core/internal/shared/database"
	"github.com/waydxd/Orbit-core/internal/shared/models"
)

// Repository defines database operations for locations
type Repository interface {
	CreateLocation(ctx context.Context, location *models.Location) error
	GetLocationByID(ctx context.Context, id string) (*models.Location, error)
	GetLocationHistory(ctx context.Context, userID string, limit int) ([]*models.Location, error)
	GetCurrentLocation(ctx context.Context, userID string) (*models.Location, error)
	FindNearby(ctx context.Context, latitude, longitude, radiusKm float64) ([]*models.Location, error)
}

// SQLRepository implements Repository using PostgreSQL
type SQLRepository struct {
	db *database.DB
}

// NewSQLRepository creates a new SQL repository
func NewSQLRepository(db *database.DB) Repository {
	return &SQLRepository{db: db}
}

// CreateLocation inserts a new location into the database
func (r *SQLRepository) CreateLocation(ctx context.Context, location *models.Location) error {
	query := `
		INSERT INTO locations (id, user_id, latitude, longitude, address, timestamp, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.ExecContext(ctx, query,
		location.ID,
		location.UserID,
		location.Latitude,
		location.Longitude,
		location.Address,
		location.Timestamp,
		location.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create location: %w", err)
	}
	return nil
}

// GetLocationByID retrieves a location by ID
func (r *SQLRepository) GetLocationByID(ctx context.Context, id string) (*models.Location, error) {
	query := `
		SELECT id, user_id, latitude, longitude, address, timestamp, created_at
		FROM locations WHERE id = $1
	`
	location := &models.Location{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&location.ID,
		&location.UserID,
		&location.Latitude,
		&location.Longitude,
		&location.Address,
		&location.Timestamp,
		&location.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("location not found")
		}
		return nil, fmt.Errorf("failed to get location: %w", err)
	}
	return location, nil
}

// GetLocationHistory retrieves recent location history for a user
func (r *SQLRepository) GetLocationHistory(ctx context.Context, userID string, limit int) ([]*models.Location, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `
		SELECT id, user_id, latitude, longitude, address, timestamp, created_at
		FROM locations
		WHERE user_id = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`
	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get location history: %w", err)
	}
	defer rows.Close()

	var locations []*models.Location
	for rows.Next() {
		location := &models.Location{}
		err := rows.Scan(
			&location.ID,
			&location.UserID,
			&location.Latitude,
			&location.Longitude,
			&location.Address,
			&location.Timestamp,
			&location.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan location: %w", err)
		}
		locations = append(locations, location)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating locations: %w", err)
	}

	return locations, nil
}

// GetCurrentLocation retrieves the most recent location for a user
func (r *SQLRepository) GetCurrentLocation(ctx context.Context, userID string) (*models.Location, error) {
	query := `
		SELECT id, user_id, latitude, longitude, address, timestamp, created_at
		FROM locations
		WHERE user_id = $1
		ORDER BY timestamp DESC
		LIMIT 1
	`
	location := &models.Location{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&location.ID,
		&location.UserID,
		&location.Latitude,
		&location.Longitude,
		&location.Address,
		&location.Timestamp,
		&location.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no location found for user")
		}
		return nil, fmt.Errorf("failed to get current location: %w", err)
	}
	return location, nil
}

// FindNearby finds locations near a given coordinate within a radius
// Uses PostgreSQL PostGIS if available, or a basic distance calculation
func (r *SQLRepository) FindNearby(ctx context.Context, latitude, longitude, radiusKm float64) ([]*models.Location, error) {
	// Basic haversine distance calculation
	// Distance is in kilometers: ACOS(SIN(lat1)*SIN(lat2) + COS(lat1)*COS(lat2)*COS(lon2-lon1)) * 6371
	query := `
		SELECT id, user_id, latitude, longitude, address, timestamp, created_at
		FROM locations
		WHERE ACOS(
			SIN(RADIANS(latitude)) * SIN(RADIANS($1)) +
			COS(RADIANS(latitude)) * COS(RADIANS($1)) * COS(RADIANS($3 - longitude))
		) * 6371 <= $2
		ORDER BY timestamp DESC
		LIMIT 100
	`
	rows, err := r.db.QueryContext(ctx, query, latitude, radiusKm, longitude)
	if err != nil {
		return nil, fmt.Errorf("failed to find nearby locations: %w", err)
	}
	defer rows.Close()

	var locations []*models.Location
	for rows.Next() {
		location := &models.Location{}
		err := rows.Scan(
			&location.ID,
			&location.UserID,
			&location.Latitude,
			&location.Longitude,
			&location.Address,
			&location.Timestamp,
			&location.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan location: %w", err)
		}
		locations = append(locations, location)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating locations: %w", err)
	}

	return locations, nil
}
