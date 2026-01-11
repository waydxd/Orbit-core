package location

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockLocationRepository is a mock for the ILocationRepository interface.
type MockLocationRepository struct {
	TrackedLocations []TrackedLocation
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

// --- Location Service Tests ---

// LocationService handles location-related operations.
type LocationService struct {
	repo ILocationRepository // Assuming ILocationRepository interface exists
}

// NewLocationService creates a new LocationService.
func NewLocationService(repo ILocationRepository) *LocationService {
	return &LocationService{
		repo: repo,
	}
}

// TrackUserLocation tests the service's ability to track location.
func (s *LocationService) TrackUserLocation(ctx context.Context, userID string, latitude, longitude float64) error {
	// Basic validation
	if userID == "" {
		return errors.New("userID cannot be empty")
	}
	if latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
		return errors.New("invalid coordinates")
	}

	// Call repository method
	err := s.repo.TrackLocation(ctx, userID, latitude, longitude)
	if err != nil {
		return err // Propagate repository error
	}
	return nil
}

// GetUserLocationHistory tests retrieving location history.
func (s *LocationService) GetUserLocationHistory(ctx context.Context, userID string) ([]TrackedLocation, error) {
	if userID == "" {
		return nil, errors.New("userID cannot be empty")
	}
	history, err := s.repo.GetLocationHistory(ctx, userID)
	if err != nil {
		return nil, err // Propagate repository error
	}
	return history, nil
}

// GetUserLatestLocation tests retrieving the latest location.
func (s *LocationService) GetUserLatestLocation(ctx context.Context, userID string) (*TrackedLocation, error) {
	if userID == "" {
		return nil, errors.New("userID cannot be empty")
	}
	latest, err := s.repo.GetLatestLocation(ctx, userID)
	if err != nil {
		return nil, err // Propagate repository error
	}
	return latest, nil
}

// --- Test Cases for Location Service ---

func TestLocationService_TrackUserLocation(t *testing.T) {
	mockRepo := NewMockLocationRepository()
	svc := NewLocationService(mockRepo)
	ctx := context.Background()

	// Test case 1: Valid location tracking
	err := svc.TrackUserLocation(ctx, "user123", 34.0522, -118.2437) // Los Angeles coords
	if err != nil {
		t.Fatalf("TrackUserLocation failed: %v", err)
	}
	if len(mockRepo.TrackedLocations) != 1 {
		t.Errorf("Expected 1 tracked location, got %d", len(mockRepo.TrackedLocations))
	}

	// Test case 2: Empty UserID
	err = svc.TrackUserLocation(ctx, "", 0.0, 0.0)
	if err == nil {
		t.Error("TrackUserLocation with empty userID should return error, but got nil")
	}
	if err.Error() != "userID cannot be empty" {
		t.Errorf("TrackUserLocation with empty userID returned wrong error: %v", err)
	}

	// Test case 3: Invalid coordinates
	err = svc.TrackUserLocation(ctx, "user123", 100.0, 0.0) // Invalid latitude
	if err == nil {
		t.Error("TrackUserLocation with invalid latitude should return error, but got nil")
	}
	if err.Error() != "invalid coordinates" {
		t.Errorf("TrackUserLocation with invalid latitude returned wrong error: %v", err)
	}

	err = svc.TrackUserLocation(ctx, "user123", 0.0, 200.0) // Invalid longitude
	if err == nil {
		t.Error("TrackUserLocation with invalid longitude should return error, but got nil")
	}
	if err.Error() != "invalid coordinates" {
		t.Errorf("TrackUserLocation with invalid longitude returned wrong error: %v", err)
	}

	// Test case 4: Repository error
	mockRepo.TrackLocation = func(ctx context.Context, userID string, latitude, longitude float64) error {
		return errors.New("repository track error")
	}
	err = svc.TrackUserLocation(ctx, "user123", 1.0, 1.0)
	if err == nil {
		t.Error("TrackUserLocation should return error when repository fails, but got nil")
	}
	if err.Error() != "repository track error" {
		t.Errorf("TrackUserLocation returned wrong error: %v", err)
	}
}

func TestLocationService_GetUserLocationHistory(t *testing.T) {
	mockRepo := NewMockLocationRepository()
	svc := NewLocationService(mockRepo)
	ctx := context.Background()
	userID := "history-user"

	// Populate repo with mock data
	mockRepo.TrackedLocations = []TrackedLocation{
		{UserID: userID, Latitude: 10.0, Longitude: 10.0, Timestamp: time.Now().Add(-time.Hour)},
		{UserID: userID, Latitude: 11.0, Longitude: 11.0, Timestamp: time.Now()},
		{UserID: "other-user", Latitude: 20.0, Longitude: 20.0, Timestamp: time.Now()},
	}

	// Test case 1: User has history
	history, err := svc.GetUserLocationHistory(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserLocationHistory failed: %v", err)
	}
	if len(history) != 2 {
		t.Errorf("Expected 2 locations for user %q, got %d", userID, len(history))
	}

	// Test case 2: User has no history
	_, err = svc.GetUserLocationHistory(ctx, "no-history-user")
	if err == nil {
		t.Error("GetUserLocationHistory for user with no history should return error, but got nil")
	}
	if err != nil && err.Error() != "no location history found for user" {
		t.Errorf("GetUserLocationHistory for user with no history returned wrong error: %v", err)
	}

	// Test case 3: Repository error
	mockRepo.GetLocationHistory = func(ctx context.Context, userID string) ([]TrackedLocation, error) {
		return nil, errors.New("repository history error")
	}
	_, err = svc.GetUserLocationHistory(ctx, userID)
	if err == nil {
		t.Error("GetUserLocationHistory should return error when repository fails, but got nil")
	}
	if err.Error() != "repository history error" {
		t.Errorf("GetUserLocationHistory returned wrong error: %v", err)
	}
}

func TestLocationService_GetUserLatestLocation(t *testing.T) {
	mockRepo := NewMockLocationRepository()
	svc := NewLocationService(mockRepo)
	ctx := context.Background()
	userID := "latest-user"
	now := time.Now()

	mockRepo.TrackedLocations = []TrackedLocation{
		{UserID: userID, Latitude: 1.0, Longitude: 1.0, Timestamp: now.Add(-time.Hour)},
		{UserID: userID, Latitude: 2.0, Longitude: 2.0, Timestamp: now}, // Latest
		{UserID: "other-user", Latitude: 3.0, Longitude: 3.0, Timestamp: now.Add(-30 * time.Minute)},
	}

	// Test case 1: User has latest location
	latest, err := svc.GetUserLatestLocation(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserLatestLocation failed for user %q: %v", userID, err)
	}
	if latest == nil {
		t.Fatal("GetUserLatestLocation returned nil for user")
	}
	if latest.Latitude != 2.0 || latest.Longitude != 2.0 {
		t.Errorf("GetUserLatestLocation for user %q returned incorrect coords: got (%.1f, %.1f), want (2.0, 2.0)", userID, latest.Latitude, latest.Longitude)
	}

	// Test case 2: User has no latest location
	_, err = svc.GetUserLatestLocation(ctx, "no-latest-user")
	if err == nil {
		t.Error("GetUserLatestLocation for user with no latest location should return error, but got nil")
	}
	if err != nil && err.Error() != "latest location not found for user" {
		t.Errorf("GetUserLatestLocation for user with no latest location returned wrong error: %v", err)
	}

	// Test case 3: Repository error
	mockRepo.GetLatestLocation = func(ctx context.Context, userID string) (*TrackedLocation, error) {
		return nil, errors.New("repository latest location error")
	}
	_, err = svc.GetUserLatestLocation(ctx, userID)
	if err == nil {
		t.Error("GetUserLatestLocation should return error when repository fails, but got nil")
	}
	if err.Error() != "repository latest location error" {
		t.Errorf("GetUserLatestLocation returned wrong error: %v", err)
	}
}

// NOTE: The 'TrackedLocation' struct and 'ILocationRepository' interface need to be defined
// and imported from the 'location' package.
// These mocks and tests are illustrative and assume basic method signatures and return types.
