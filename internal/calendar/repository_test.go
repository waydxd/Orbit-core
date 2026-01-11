package calendar

import (
	"context"
	"testing"
	"time"
)

// MockCalendarRepository is a mock for the ICalendarRepository interface.
// It simulates database operations using an in-memory slice.
type MockCalendarRepository struct {
	events []Event // Assuming Event struct exists and is defined elsewhere
}

// NewMockCalendarRepository creates a new mock repository.
func NewMockCalendarRepository() *MockCalendarRepository {
	return &MockCalendarRepository{
		events: []Event{},
	}
}

// GetEvents simulates retrieving events.
func (r *MockCalendarRepository) GetEvents(ctx context.Context, userID string, queryParams map[string]string) ([]Event, error) {
	// In a real mock, you'd filter events based on userID and queryParams.
	// For now, return a dummy event if any events exist, or an empty list.
	if len(r.events) > 0 {
		// Return a copy to prevent external modification of the mock's internal state
		// return []Event{r.events[0]}, nil // Simplified
		return r.events, nil // Returning all for now
	}
	return []Event{}, nil
}

// CreateEvent simulates creating a new event.
func (r *MockCalendarRepository) CreateEvent(ctx context.Context, event *Event) (*Event, error) {
	// Simulate assigning an ID and setting timestamps
	event.ID = "mock-event-id-" + time.Now().Format("20060102150405") // Simple mock ID
	event.CreatedAt = time.Now()
	event.UpdatedAt = time.Now()
	r.events = append(r.events, *event)
	return event, nil
}

// GetEventByID simulates retrieving a single event by its ID.
func (r *MockCalendarRepository) GetEventByID(ctx context.Context, eventID string) (*Event, error) {
	for _, e := range r.events {
		if e.ID == eventID {
			return &e, nil
		}
	}
	return nil, errors.New("event not found") // Simulate not found
}

// UpdateEvent simulates updating an existing event.
func (r *MockCalendarRepository) UpdateEvent(ctx context.Context, eventID string, event *Event) (*Event, error) {
	for i, e := range r.events {
		if e.ID == eventID {
			// Update fields, keeping ID, CreatedAt, and setting UpdatedAt
			event.ID = e.ID // Ensure ID is not changed
			event.CreatedAt = e.CreatedAt
			event.UpdatedAt = time.Now()
			r.events[i] = *event
			return &r.events[i], nil
		}
	}
	return nil, errors.New("event not found")
}

// DeleteEvent simulates deleting an event.
func (r *MockCalendarRepository) DeleteEvent(ctx context.Context, eventID string) error {
	for i, e := range r.events {
		if e.ID == eventID {
			r.events = append(r.events[:i], r.events[i+1:]...)
			return nil
		}
	}
	return errors.New("event not found")
}

// --- Test Cases for Repository ---

func TestMockCalendarRepository_GetEvents(t *testing.T) {
	repo := NewMockCalendarRepository()
	ctx := context.Background()
	// Add some mock events
	repo.events = []Event{
		{ID: "1", UserID: "user1", Title: "Meeting", StartTime: time.Now(), EndTime: time.Now().Add(time.Hour)},
		{ID: "2", UserID: "user1", Title: "Lunch", StartTime: time.Now().Add(2 * time.Hour), EndTime: time.Now().Add(3 * time.Hour)},
	}

	events, err := repo.GetEvents(ctx, "user1", nil)
	if err != nil {
		t.Fatalf("GetEvents failed: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}
}

func TestMockCalendarRepository_CreateEvent(t *testing.T) {
	repo := NewMockCalendarRepository()
	ctx := context.Background()
	newEvent := &Event{UserID: "user1", Title: "New Event", StartTime: time.Now(), EndTime: time.Now().Add(time.Hour)}

	createdEvent, err := repo.CreateEvent(ctx, newEvent)
	if err != nil {
		t.Fatalf("CreateEvent failed: %v", err)
	}
	if createdEvent == nil {
		t.Fatal("CreateEvent returned nil event")
	}
	if createdEvent.ID == "" || createdEvent.CreatedAt.IsZero() || createdEvent.UpdatedAt.IsZero() {
		t.Error("CreateEvent did not properly set ID or timestamps")
	}
	if len(repo.events) != 1 {
		t.Errorf("Expected 1 event after CreateEvent, got %d", len(repo.events))
	}
}

func TestMockCalendarRepository_GetEventByID(t *testing.T) {
	repo := NewMockCalendarRepository()
	ctx := context.Background()
	eventID := "unique-event-id"
	repo.events = []Event{{ID: eventID, Title: "Event by ID"}}

	// Test case 1: Event exists
	event, err := repo.GetEventByID(ctx, eventID)
	if err != nil {
		t.Fatalf("GetEventByID(%q) failed: %v", eventID, err)
	}
	if event == nil || event.ID != eventID {
		t.Errorf("GetEventByID(%q) returned wrong event or nil", eventID)
	}

	// Test case 2: Event does not exist
	nonExistentID := "non-existent-id"
	_, err = repo.GetEventByID(ctx, nonExistentID)
	if err == nil {
		t.Errorf("GetEventByID(%q) should return an error, but got nil", nonExistentID)
	}
}

func TestMockCalendarRepository_UpdateEvent(t *testing.T) {
	repo := NewMockCalendarRepository()
	ctx := context.Background()
	eventID := "update-event-id"
	originalEvent := &Event{ID: eventID, Title: "Original Title", UserID: "user1", StartTime: time.Now(), EndTime: time.Now().Add(time.Hour)}
	repo.events = append(repo.events, *originalEvent)

	updatedEventData := &Event{Title: "Updated Title", UserID: "user1", StartTime: time.Now().Add(time.Hour), EndTime: time.Now().Add(2 * time.Hour)}
	
	// Test case 1: Event exists
	updatedEvent, err := repo.UpdateEvent(ctx, eventID, updatedEventData)
	if err != nil {
		t.Fatalf("UpdateEvent(%q) failed: %v", eventID, err)
	}
	if updatedEvent == nil || updatedEvent.ID != eventID {
		t.Errorf("UpdateEvent(%q) returned wrong event or nil", eventID)
	}
	if updatedEvent.Title != "Updated Title" {
		t.Errorf("UpdateEvent(%q) title not updated: got %q, want %q", eventID, updatedEvent.Title, "Updated Title")
	}
	if updatedEvent.UpdatedAt.Before(updatedEvent.CreatedAt) {
		t.Errorf("UpdateEvent(%q) UpdatedAt is before CreatedAt", eventID)
	}

	// Test case 2: Event does not exist
	nonExistentID := "non-existent-update-id"
	_, err = repo.UpdateEvent(ctx, nonExistentID, &Event{})
	if err == nil {
		t.Errorf("UpdateEvent(%q) should return an error, but got nil", nonExistentID)
	}
}

func TestMockCalendarRepository_DeleteEvent(t *testing.T) {
	repo := NewMockCalendarRepository()
	ctx := context.Background()
	eventID := "delete-event-id"
	repo.events = []Event{{ID: eventID, Title: "Event to Delete"}}

	// Test case 1: Event exists
	err := repo.DeleteEvent(ctx, eventID)
	if err != nil {
		t.Fatalf("DeleteEvent(%q) failed: %v", eventID, err)
	}
	if len(repo.events) != 0 {
		t.Errorf("Expected 0 events after delete, got %d", len(repo.events))
	}

	// Test case 2: Event does not exist
	nonExistentID := "non-existent-delete-id"
	err = repo.DeleteEvent(ctx, nonExistentID)
	if err == nil {
		t.Errorf("DeleteEvent(%q) should return an error, but got nil", nonExistentID)
	}
}

// NOTE: The 'Event' struct and 'ICalendarRepository' interface need to be defined
// and imported from the 'calendar' package for these tests to compile and run correctly.
// This mock implementation assumes their basic structure.
