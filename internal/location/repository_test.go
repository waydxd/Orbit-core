package location

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockLocationRepository is a mock implementation for ILocationRepository.
type MockLocationRepository struct {
	TrackedLocations []TrackedLocation // Assuming TrackedLocation struct exists
}

// TrackedLocation represents a mock location entry.
type TrackedLocation struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Timestamp time.Time `json:"timestamp"`
}

// NewMockLocationRepository creates a new mock repository.
func NewMockLocationRepository() *MockLocationRepository {
	return &MockLocationRepository{
		TrackedLocations: []TrackedLocation{},
	}
}

// TrackLocation simulates saving a new location entry.
func (r *MockLocationRepository) TrackLocation(ctx context.Context, userID string, latitude, longitude float64) error {
	newLocation := TrackedLocation{
		ID:        "mock-loc-id-" + time.Now().Format("20060102150405"), // Simple mock ID
		UserID:    userID,
		Latitude:  latitude,
		Longitude: longitude,
		Timestamp: time.Now(),
	}
	r.TrackedLocations = append(r.TrackedLocations, newLocation)
	return nil
}

// GetLocationHistory simulates retrieving location history for a user.
func (r *MockLocationRepository) GetLocationHistory(ctx context.Context, userID string) ([]TrackedLocation, error) {
	var userLocations []TrackedLocation
	for _, loc := range r.TrackedLocations {
		if loc.UserID == userID {
			userLocations = append(userLocations, loc)
		}
	}
	if len(userLocations) == 0 {
		return nil, errors.New("no location history found for user")
	}
	return userLocations, nil
}

// GetLatestLocation simulates retrieving the most recent location for a user.
func (r *MockLocationRepository) GetLatestLocation(ctx context.Context, userID string) (*TrackedLocation, error) {
	var latestLocation *TrackedLocation
	var latestTimestamp time.Time

	for _, loc := range r.TrackedLocations {
		if loc.UserID == userID {
			if latestLocation == nil || loc.Timestamp.After(latestTimestamp) {
				latestLocation = &loc
				latestTimestamp = loc.Timestamp
			}
		}
	}

	if latestLocation == nil {
		return nil, errors.New("latest location not found for user")
	}
	return latestLocation, nil
}

// --- Test Cases for Location Repository ---

func TestMockLocationRepository_TrackLocation(t *testing.T) {
	repo := NewMockLocationRepository()
	ctx := context.Background()
	userID := "user1"
	lat, lon := 40.7128, -74.0060 // New York City coordinates

	err := repo.TrackLocation(ctx, userID, lat, lon)
	if err != nil {
		t.Fatalf("TrackLocation failed: %v", err)
	}

	if len(repo.TrackedLocations) != 1 {
		t.Errorf("Expected 1 tracked location, got %d", len(repo.TrackedLocations))
	}
	location := repo.TrackedLocations[0]
	if location.UserID != userID || location.Latitude != lat || location.Longitude != lon || location.Timestamp.IsZero() {
		t.Errorf("TrackLocation stored incorrect data: %+v", location)
	}
}

func TestMockLocationRepository_GetLocationHistory(t *testing.T) {
	repo := NewMockLocationRepository()
	ctx := context.Background()
	repo.TrackedLocations = []TrackedLocation{
		{UserID: "user1", Latitude: 1.0, Longitude: 1.0, Timestamp: time.Now().Add(-time.Hour)},
		{UserID: "user1", Latitude: 2.0, Longitude: 2.0, Timestamp: time.Now()},
		{UserID: "user2", Latitude: 3.0, Longitude: 3.0, Timestamp: time.Now()},
	}

	// Test case 1: User has history
	history, err := repo.GetLocationHistory(ctx, "user1")
	if err != nil {
		t.Fatalf("GetLocationHistory failed: %v", err)
	}
	if len(history) != 2 {
		t.Errorf("Expected 2 locations for user1, got %d", len(history))
	}

	// Test case 2: User has no history
	_, err = repo.GetLocationHistory(ctx, "user3")
	if err == nil {
		t.Error("GetLocationHistory for user3 should return an error, but got nil")
	}
	if err != nil && err.Error() != "no location history found for user" {
		t.Errorf("GetLocationHistory for user3 returned wrong error: %v", err)
	}
}

func TestMockLocationRepository_GetLatestLocation(t *testing.T) {
	repo := NewMockLocationRepository()
	ctx := context.Background()
	now := time.Now()
	repo.TrackedLocations = []TrackedLocation{
		{UserID: "user1", Latitude: 1.0, Longitude: 1.0, Timestamp: now.Add(-time.Hour)},
		{UserID: "user1", Latitude: 2.0, Longitude: 2.0, Timestamp: now}, // Latest
		{UserID: "user2", Latitude: 3.0, Longitude: 3.0, Timestamp: now.Add(-30 * time.Minute)},
	}

	// Test case 1: User has latest location
	latest, err := repo.GetLatestLocation(ctx, "user1")
	if err != nil {
		t.Fatalf("GetLatestLocation failed for user1: %v", err)
	}
	if latest == nil {
		t.Fatal("GetLatestLocation returned nil for user1")
	}
	if latest.Latitude != 2.0 || latest.Longitude != 2.0 {
		t.Errorf("GetLatestLocation for user1 returned incorrect coords: got (%.1f, %.1f), want (2.0, 2.0)", latest.Latitude, latest.Longitude)
	}

	// Test case 2: User has no latest location
	_, err = repo.GetLatestLocation(ctx, "user3")
	if err == nil {
		t.Error("GetLatestLocation for user3 should return an error, but got nil")
	}
	if err != nil && err.Error() != "latest location not found for user" {
		t.Errorf("GetLatestLocation for user3 returned wrong error: %v", err)
	}
}

// NOTE: The 'TrackedLocation' struct and 'ILocationRepository' interface need to be defined
// and imported from the 'location' package.
// These mocks and tests are illustrative and assume basic method signatures and return types.
