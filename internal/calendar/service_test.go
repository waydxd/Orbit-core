package calendar

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockEvent represents a dummy event for testing.
// This should mirror the actual Event struct defined in the calendar package.
type Event struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Title     string    `json:"title"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// MockICalendarRepository is an interface for mocking repository calls.
type MockICalendarRepository interface {
	GetEvents(ctx context.Context, userID string, queryParams map[string]string) ([]Event, error)
	CreateEvent(ctx context.Context, event *Event) (*Event, error)
	GetEventByID(ctx context.Context, eventID string) (*Event, error)
	UpdateEvent(ctx context.Context, eventID string, event *Event) (*Event, error)
	DeleteEvent(ctx context.Context, eventID string) error
}

// CalendarService represents the service layer for calendar operations.
type CalendarService struct {
	repo ICalendarRepository // Use the actual interface here, not the mock
}

// NewCalendarService creates a new CalendarService.
// It should accept an ICalendarRepository implementation.
func NewCalendarService(repo ICalendarRepository) *CalendarService {
	return &CalendarService{
		repo: repo,
	}
}

// --- Test Cases for Service ---

func TestCalendarService_GetEvents(t *testing.T) {
	// Mock repository setup
	mockRepo := &MockCalendarRepository{} // Using the mock defined in repository_test.go for simulation
	svc := NewCalendarService(mockRepo)
	ctx := context.Background()
	userID := "testuser123"

	// Populate mock repo with some data
	mockRepo.events = []Event{
		{ID: "e1", UserID: userID, Title: "Meeting", StartTime: time.Now(), EndTime: time.Now().Add(time.Hour)},
		{ID: "e2", UserID: userID, Title: "Lunch", StartTime: time.Now().Add(2 * time.Hour), EndTime: time.Now().Add(3 * time.Hour)},
		{ID: "e3", UserID: "anotheruser", Title: "Other Event", StartTime: time.Now(), EndTime: time.Now().Add(time.Hour)},
	}

	// Test case 1: User has events
	queryParams := map[string]string{"user_id": userID} // Example query param
	events, err := svc.GetEvents(ctx, userID, queryParams)
	if err != nil {
		t.Fatalf("GetEvents failed: %v", err)
	}
	if len(events) != 2 { // Expecting only events for 'userID'
		t.Errorf("GetEvents for user %q returned %d events, want 2", userID, len(events))
	}

	// Test case 2: User has no events (or repo returns empty)
	mockRepo.events = []Event{} // Clear mock data
	events, err = svc.GetEvents(ctx, "nonexistentuser", nil)
	if err != nil {
		t.Fatalf("GetEvents for non-existent user failed: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("GetEvents for non-existent user returned %d events, want 0", len(events))
	}

	// Test case 3: Repository returns an error
	mockRepo.GetEvents = func(ctx context.Context, userID string, queryParams map[string]string) ([]Event, error) {
		return nil, errors.New("repository error")
	}
	_, err = svc.GetEvents(ctx, userID, nil)
	if err == nil {
		t.Error("GetEvents should return an error when repository returns error, but got nil")
	}
	if err.Error() != "repository error" {
		t.Errorf("GetEvents returned wrong error message: got %q, want %q", err.Error(), "repository error")
	}
}

func TestCalendarService_CreateEvent(t *testing.T) {
	mockRepo := &MockCalendarRepository{}
	svc := NewCalendarService(mockRepo)
	ctx := context.Background()

	newEventData := &Event{
		UserID:    "user456",
		Title:     "New Task",
		StartTime: time.Now().Add(time.Hour),
		EndTime:   time.Now().Add(2 * time.Hour),
	}

	// Test case 1: Successful creation
	createdEvent, err := svc.CreateEvent(ctx, newEventData)
	if err != nil {
		t.Fatalf("CreateEvent failed: %v", err)
	}
	if createdEvent == nil {
		t.Fatal("CreateEvent returned nil event")
	}
	if createdEvent.ID == "" {
		t.Error("CreateEvent did not assign an ID to the event")
	}
	if createdEvent.UserID != newEventData.UserID || createdEvent.Title != newEventData.Title {
		t.Error("CreateEvent returned event with incorrect data")
	}
	if createdEvent.CreatedAt.IsZero() || createdEvent.UpdatedAt.IsZero() {
		t.Error("CreateEvent did not set CreatedAt or UpdatedAt timestamps")
	}

	// Test case 2: Repository returns an error
	mockRepo.CreateEvent = func(ctx context.Context, event *Event) (*Event, error) {
		return nil, errors.New("repository creation error")
	}
	_, err = svc.CreateEvent(ctx, newEventData)
	if err == nil {
		t.Error("CreateEvent should return an error when repository fails, but got nil")
	}
	if err.Error() != "repository creation error" {
		t.Errorf("CreateEvent returned wrong error message: got %q, want %q", err.Error(), "repository creation error")
	}
}

func TestCalendarService_GetEventByID(t *testing.T) {
	mockRepo := &MockCalendarRepository{}
	svc := NewCalendarService(mockRepo)
	ctx := context.Background()
	eventID := "event-to-fetch-id"
	mockEvent := &Event{ID: eventID, UserID: "user789", Title: "Fetch Me"}
	mockRepo.events = append(mockRepo.events, *mockEvent)

	// Test case 1: Event found
	event, err := svc.GetEventByID(ctx, eventID)
	if err != nil {
		t.Fatalf("GetEventByID(%q) failed: %v", eventID, err)
	}
	if event == nil || event.ID != eventID {
		t.Errorf("GetEventByID(%q) returned wrong event or nil", eventID)
	}

	// Test case 2: Event not found
	nonExistentID := "non-existent-event-id"
	_, err = svc.GetEventByID(ctx, nonExistentID)
	if err == nil {
		t.Errorf("GetEventByID(%q) should return an error, but got nil", nonExistentID)
	}
	// Check for the expected error from the mock repository
	if err != nil && err.Error() != "event not found" {
		t.Errorf("GetEventByID(%q) returned unexpected error: %v", nonExistentID, err)
	}

	// Test case 3: Repository returns an error
	mockRepo.GetEventByID = func(ctx context.Context, eventID string) (*Event, error) {
		return nil, errors.New("repository lookup error")
	}
	_, err = svc.GetEventByID(ctx, eventID)
	if err == nil {
		t.Error("GetEventByID should return an error when repository fails, but got nil")
	}
	if err.Error() != "repository lookup error" {
		t.Errorf("GetEventByID returned wrong error message: got %q, want %q", err.Error(), "repository lookup error")
	}
}

// Add tests for UpdateEvent and DeleteEvent similarly, mocking repository calls.

// NOTE: The 'Event' struct needs to be defined and imported from the 'calendar' package.
// The 'ICalendarRepository' interface also needs to be defined.
// These tests are illustrative and assume basic method signatures and return types.
