package google

import (
	"context"
	"testing"
	"time"

	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
	"golang.org/x/oauth2"
	gcal "google.golang.org/api/calendar/v3"
)

func TestInMemoryTokenStore(t *testing.T) {
	store := NewInMemoryTokenStore()
	ctx := context.Background()
	userID := "test-user-123"

	// Test GetToken when no token exists
	_, err := store.GetToken(ctx, userID)
	if err == nil {
		t.Error("Expected error when getting non-existent token")
	}

	// Test SaveToken
	token := &oauth2.Token{
		AccessToken:  "access-token-123",
		RefreshToken: "refresh-token-456",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
	err = store.SaveToken(ctx, userID, token)
	if err != nil {
		t.Fatalf("SaveToken failed: %v", err)
	}

	// Test GetToken after saving
	retrievedToken, err := store.GetToken(ctx, userID)
	if err != nil {
		t.Fatalf("GetToken failed: %v", err)
	}
	if retrievedToken.AccessToken != token.AccessToken {
		t.Errorf("Expected access token %s, got %s", token.AccessToken, retrievedToken.AccessToken)
	}

	// Test DeleteToken
	err = store.DeleteToken(ctx, userID)
	if err != nil {
		t.Fatalf("DeleteToken failed: %v", err)
	}

	// Verify token is deleted
	_, err = store.GetToken(ctx, userID)
	if err == nil {
		t.Error("Expected error after deleting token")
	}
}

func TestServiceIsConfigured(t *testing.T) {
	log := logger.New()

	testCases := []struct {
		name      string
		clientID  string
		clientSec string
		expected  bool
	}{
		{"Not configured - empty", "", "", false},
		{"Not configured - missing secret", "client-id", "", false},
		{"Not configured - missing id", "", "client-secret", false},
		{"Configured", "client-id", "client-secret", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				GoogleCalendar: config.GoogleCalendarConfig{
					ClientID:     tc.clientID,
					ClientSecret: tc.clientSec,
					RedirectURL:  "http://localhost/callback",
				},
			}

			tokenStore := NewInMemoryTokenStore()
			svc := NewService(cfg, log, tokenStore)

			if svc.IsConfigured() != tc.expected {
				t.Errorf("IsConfigured() = %v, expected %v", svc.IsConfigured(), tc.expected)
			}
		})
	}
}

func TestGetAuthURL(t *testing.T) {
	log := logger.New()

	// Test with unconfigured service
	cfg := &config.Config{
		GoogleCalendar: config.GoogleCalendarConfig{},
	}
	tokenStore := NewInMemoryTokenStore()
	svc := NewService(cfg, log, tokenStore)

	_, err := svc.GetAuthURL("user123")
	if err == nil {
		t.Error("Expected error when service is not configured")
	}

	// Test with configured service
	cfg = &config.Config{
		GoogleCalendar: config.GoogleCalendarConfig{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			RedirectURL:  "http://localhost/callback",
		},
	}
	svc = NewService(cfg, log, tokenStore)

	url, err := svc.GetAuthURL("user123")
	if err != nil {
		t.Fatalf("GetAuthURL failed: %v", err)
	}

	if url == "" {
		t.Error("Expected non-empty auth URL")
	}

	// URL should contain the client ID
	if !containsString(url, "test-client-id") {
		t.Error("Auth URL should contain client ID")
	}
}

func TestGoogleEventToModel(t *testing.T) {
	userID := "user123"

	// Test nil event
	result := googleEventToModel(nil, userID)
	if result != nil {
		t.Error("Expected nil result for nil event")
	}

	// Test event with empty summary
	emptyEvent := &gcal.Event{
		Summary: "",
	}
	result = googleEventToModel(emptyEvent, userID)
	if result != nil {
		t.Error("Expected nil result for event with empty summary")
	}

	// Test valid event with DateTime
	validEvent := &gcal.Event{
		Id:          "google-event-123",
		Summary:     "Test Meeting",
		Description: "Meeting description",
		Location:    "Conference Room",
		Start: &gcal.EventDateTime{
			DateTime: "2024-01-15T10:00:00Z",
		},
		End: &gcal.EventDateTime{
			DateTime: "2024-01-15T11:00:00Z",
		},
	}
	result = googleEventToModel(validEvent, userID)
	if result == nil {
		t.Fatal("Expected non-nil result for valid event")
	}
	if result.Title != "Test Meeting" {
		t.Errorf("Expected title 'Test Meeting', got '%s'", result.Title)
	}
	if result.Description != "Meeting description" {
		t.Errorf("Expected description 'Meeting description', got '%s'", result.Description)
	}
	if result.Location != "Conference Room" {
		t.Errorf("Expected location 'Conference Room', got '%s'", result.Location)
	}
	if result.UserID != userID {
		t.Errorf("Expected userID '%s', got '%s'", userID, result.UserID)
	}

	// Test event with Date only (all-day event)
	allDayEvent := &gcal.Event{
		Summary: "All Day Event",
		Start: &gcal.EventDateTime{
			Date: "2024-01-15",
		},
		End: &gcal.EventDateTime{
			Date: "2024-01-16",
		},
	}
	result = googleEventToModel(allDayEvent, userID)
	if result == nil {
		t.Fatal("Expected non-nil result for all-day event")
	}
	if result.Title != "All Day Event" {
		t.Errorf("Expected title 'All Day Event', got '%s'", result.Title)
	}
}

func TestModelToGoogleEvent(t *testing.T) {
	event := &models.Event{
		ID:          "event-123",
		UserID:      "user456",
		Title:       "Local Event",
		Description: "Event description",
		Location:    "Office",
		StartTime:   time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		EndTime:     time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
	}

	gEvent := modelToGoogleEvent(event)

	if gEvent.Summary != "Local Event" {
		t.Errorf("Expected summary 'Local Event', got '%s'", gEvent.Summary)
	}
	if gEvent.Description != "Event description" {
		t.Errorf("Expected description 'Event description', got '%s'", gEvent.Description)
	}
	if gEvent.Location != "Office" {
		t.Errorf("Expected location 'Office', got '%s'", gEvent.Location)
	}
	if gEvent.Start == nil || gEvent.Start.DateTime == "" {
		t.Error("Expected Start.DateTime to be set")
	}
	if gEvent.End == nil || gEvent.End.DateTime == "" {
		t.Error("Expected End.DateTime to be set")
	}
}

func TestHandleCallbackInvalidState(t *testing.T) {
	log := logger.New()
	cfg := &config.Config{
		GoogleCalendar: config.GoogleCalendarConfig{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			RedirectURL:  "http://localhost/callback",
		},
	}
	tokenStore := NewInMemoryTokenStore()
	svc := NewService(cfg, log, tokenStore)

	ctx := context.Background()

	// Test with invalid state
	_, err := svc.HandleCallback(ctx, "invalid-state", "some-code")
	if err == nil {
		t.Error("Expected error for invalid state")
	}
}

// Helper function
func containsString(s, substr string) bool {
	return containsSubstring(s, substr)
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
