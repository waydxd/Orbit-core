package maps

import (
	"context"
	"fmt"

	"github.com/waydxd/Orbit-core/internal/shared/models"
)

// LocationFetcher is the subset of the location repository needed by the adapter.
type LocationFetcher interface {
	GetCurrentLocation(ctx context.Context, userID string) (*models.Location, error)
}

// LocationAdapter adapts a LocationFetcher into the LocationProvider interface
// expected by the notification service.
type LocationAdapter struct {
	fetcher LocationFetcher
}

// NewLocationAdapter creates a new LocationAdapter.
func NewLocationAdapter(fetcher LocationFetcher) *LocationAdapter {
	if fetcher == nil {
		return nil
	}
	return &LocationAdapter{fetcher: fetcher}
}

// GetCurrentLocation returns the latest lat/lng for a user.
func (a *LocationAdapter) GetCurrentLocation(ctx context.Context, userID string) (float64, float64, error) {
	loc, err := a.fetcher.GetCurrentLocation(ctx, userID)
	if err != nil {
		return 0, 0, fmt.Errorf("location adapter: %w", err)
	}
	return loc.Latitude, loc.Longitude, nil
}
