package location

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/waydxd/Orbit-core/internal/shared/database"
	"github.com/waydxd/Orbit-core/internal/shared/database/db"
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
	queries *db.Queries
	pool    *database.DB
}

// NewSQLRepository creates a new SQL repository
func NewSQLRepository(pool *database.DB) Repository {
	return &SQLRepository{
		queries: db.New(pool.Pool),
		pool:    pool,
	}
}

// CreateLocation inserts a new location into the database
func (r *SQLRepository) CreateLocation(ctx context.Context, location *models.Location) error {
	params := db.CreateLocationParams{
		ID:        database.StringToUUID(location.ID),
		UserID:    database.StringToUUID(location.UserID),
		Latitude:  database.Float64ToNumeric(location.Latitude),
		Longitude: database.Float64ToNumeric(location.Longitude),
		Address:   database.StringToText(location.Address),
		Timestamp: database.TimeToTimestamptz(location.Timestamp),
		CreatedAt: database.TimeToTimestamptz(location.CreatedAt),
	}

	err := r.queries.CreateLocation(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to create location: %w", err)
	}
	return nil
}

// GetLocationByID retrieves a location by ID
func (r *SQLRepository) GetLocationByID(ctx context.Context, id string) (*models.Location, error) {
	row, err := r.queries.GetLocationByID(ctx, database.StringToUUID(id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("location not found")
		}
		return nil, fmt.Errorf("failed to get location: %w", err)
	}

	return &models.Location{
		ID:        database.UUIDToString(row.ID),
		UserID:    database.UUIDToString(row.UserID),
		Latitude:  database.NumericToFloat64(row.Latitude),
		Longitude: database.NumericToFloat64(row.Longitude),
		Address:   database.TextToString(row.Address),
		Timestamp: database.TimestamptzToTime(row.Timestamp),
		CreatedAt: database.TimestamptzToTime(row.CreatedAt),
	}, nil
}

// GetLocationHistory retrieves recent location history for a user
func (r *SQLRepository) GetLocationHistory(ctx context.Context, userID string, limit int) ([]*models.Location, error) {
	if limit <= 0 {
		limit = 100
	}
	params := db.GetLocationHistoryParams{
		UserID: database.StringToUUID(userID),
		Limit:  database.IntToInt32(limit),
	}

	rows, err := r.queries.GetLocationHistory(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get location history: %w", err)
	}

	var locations []*models.Location
	for _, row := range rows {
		locations = append(locations, &models.Location{
			ID:        database.UUIDToString(row.ID),
			UserID:    database.UUIDToString(row.UserID),
			Latitude:  database.NumericToFloat64(row.Latitude),
			Longitude: database.NumericToFloat64(row.Longitude),
			Address:   database.TextToString(row.Address),
			Timestamp: database.TimestamptzToTime(row.Timestamp),
			CreatedAt: database.TimestamptzToTime(row.CreatedAt),
		})
	}

	return locations, nil
}

// GetCurrentLocation retrieves the most recent location for a user
func (r *SQLRepository) GetCurrentLocation(ctx context.Context, userID string) (*models.Location, error) {
	row, err := r.queries.GetCurrentLocation(ctx, database.StringToUUID(userID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("no location found for user")
		}
		return nil, fmt.Errorf("failed to get current location: %w", err)
	}

	return &models.Location{
		ID:        database.UUIDToString(row.ID),
		UserID:    database.UUIDToString(row.UserID),
		Latitude:  database.NumericToFloat64(row.Latitude),
		Longitude: database.NumericToFloat64(row.Longitude),
		Address:   database.TextToString(row.Address),
		Timestamp: database.TimestamptzToTime(row.Timestamp),
		CreatedAt: database.TimestamptzToTime(row.CreatedAt),
	}, nil
}

// FindNearby finds locations near a given coordinate within a radius
func (r *SQLRepository) FindNearby(ctx context.Context, latitude, longitude, radiusKm float64) ([]*models.Location, error) {
	params := db.FindNearbyParams{
		Targetlat: latitude,
		Targetlon: longitude,
		Radius:    radiusKm,
	}

	rows, err := r.queries.FindNearby(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to find nearby locations: %w", err)
	}

	var locations []*models.Location
	for _, row := range rows {
		locations = append(locations, &models.Location{
			ID:        database.UUIDToString(row.ID),
			UserID:    database.UUIDToString(row.UserID),
			Latitude:  database.NumericToFloat64(row.Latitude),
			Longitude: database.NumericToFloat64(row.Longitude),
			Address:   database.TextToString(row.Address),
			Timestamp: database.TimestamptzToTime(row.Timestamp),
			CreatedAt: database.TimestamptzToTime(row.CreatedAt),
		})
	}

	return locations, nil
}
