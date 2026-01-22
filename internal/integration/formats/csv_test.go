package formats

import (
	"strings"
	"testing"
	"time"

	"github.com/waydxd/Orbit-core/internal/shared/models"
)

func TestParseCSV(t *testing.T) {
	csvContent := `Title,Description,Start Time,End Time,Location
Meeting 1,First meeting,2024-01-15 10:00:00,2024-01-15 11:00:00,Room A
Meeting 2,Second meeting,2024-01-16 14:00:00,2024-01-16 15:00:00,Room B`

	events, err := ParseCSV(strings.NewReader(csvContent), "user123")
	if err != nil {
		t.Fatalf("ParseCSV failed: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}

	// Check first event
	if events[0].Title != "Meeting 1" {
		t.Errorf("Expected title 'Meeting 1', got '%s'", events[0].Title)
	}
	if events[0].Description != "First meeting" {
		t.Errorf("Expected description 'First meeting', got '%s'", events[0].Description)
	}
	if events[0].Location != "Room A" {
		t.Errorf("Expected location 'Room A', got '%s'", events[0].Location)
	}
	if events[0].UserID != "user123" {
		t.Errorf("Expected userID 'user123', got '%s'", events[0].UserID)
	}

	// Check date parsing
	expectedStart := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	if !events[0].StartTime.Equal(expectedStart) {
		t.Errorf("Expected start time %v, got %v", expectedStart, events[0].StartTime)
	}
}

func TestParseCSVAlternativeHeaders(t *testing.T) {
	// Test with alternative header names (like Google Calendar export)
	csvContent := `Subject,Notes,Start Date,End Date,Where
Meeting 1,First meeting,2024-01-15 10:00:00,2024-01-15 11:00:00,Room A`

	events, err := ParseCSV(strings.NewReader(csvContent), "user123")
	if err != nil {
		t.Fatalf("ParseCSV failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].Title != "Meeting 1" {
		t.Errorf("Expected title 'Meeting 1', got '%s'", events[0].Title)
	}
	if events[0].Description != "First meeting" {
		t.Errorf("Expected description 'First meeting', got '%s'", events[0].Description)
	}
	if events[0].Location != "Room A" {
		t.Errorf("Expected location 'Room A', got '%s'", events[0].Location)
	}
}

func TestParseCSVDateFormats(t *testing.T) {
	testCases := []struct {
		name     string
		dateStr  string
		expected time.Time
	}{
		{"RFC3339", "2024-01-15T10:00:00Z", time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		{"ISO with space", "2024-01-15 10:00:00", time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		{"ISO without seconds", "2024-01-15 10:00", time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		{"Date only ISO", "2024-01-15", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"US format with time", "01/15/2024 10:00:00", time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		{"US format without seconds", "01/15/2024 10:00", time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		{"US format date only", "01/15/2024", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"EU format with time", "15/01/2024 10:00:00", time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		{"EU format without seconds", "15/01/2024 10:00", time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		{"EU format date only", "15/01/2024", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"Day first month name full", "15 Jan 2024 10:00:00", time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		{"Day first month name no seconds", "15 Jan 2024 10:00", time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		{"Day first month name date only", "15 Jan 2024", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"ISO T format no Z", "2024-01-15T10:00:00", time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			csvContent := "Title,Start Time\nTest Event," + tc.dateStr

			events, err := ParseCSV(strings.NewReader(csvContent), "user123")
			if err != nil {
				t.Fatalf("ParseCSV failed: %v", err)
			}

			if len(events) != 1 {
				t.Fatalf("Expected 1 event, got %d", len(events))
			}

			if !events[0].StartTime.Equal(tc.expected) {
				t.Errorf("Expected start time %v, got %v", tc.expected, events[0].StartTime)
			}
		})
	}
}

func TestParseCSVEmpty(t *testing.T) {
	csvContent := ""

	events, err := ParseCSV(strings.NewReader(csvContent), "user123")
	if err != nil {
		t.Fatalf("ParseCSV failed: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("Expected 0 events, got %d", len(events))
	}
}

func TestParseCSVHeaderOnly(t *testing.T) {
	csvContent := "Title,Description,Start Time,End Time,Location"

	events, err := ParseCSV(strings.NewReader(csvContent), "user123")
	if err != nil {
		t.Fatalf("ParseCSV failed: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("Expected 0 events, got %d", len(events))
	}
}

func TestGenerateCSV(t *testing.T) {
	events := []*models.Event{
		{
			ID:          "event-1",
			UserID:      "user123",
			Title:       "Test Event",
			Description: "Test description",
			Location:    "Room 101",
			StartTime:   time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			EndTime:     time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
		},
	}

	csvData, err := GenerateCSV(events)
	if err != nil {
		t.Fatalf("GenerateCSV failed: %v", err)
	}

	csvString := string(csvData)

	// Check header row
	if !strings.Contains(csvString, "Title,Description,Start Time,End Time,Location") {
		t.Error("Missing CSV header row")
	}

	// Check data row
	if !strings.Contains(csvString, "Test Event") {
		t.Error("Missing event title")
	}
	if !strings.Contains(csvString, "Test description") {
		t.Error("Missing event description")
	}
	if !strings.Contains(csvString, "Room 101") {
		t.Error("Missing event location")
	}
}

func TestGenerateCSVEmpty(t *testing.T) {
	var events []*models.Event

	csvData, err := GenerateCSV(events)
	if err != nil {
		t.Fatalf("GenerateCSV failed: %v", err)
	}

	csvString := string(csvData)

	// Should only contain header
	lines := strings.Split(strings.TrimSpace(csvString), "\n")
	if len(lines) != 1 {
		t.Errorf("Expected 1 line (header only), got %d", len(lines))
	}
}

func TestCSVRoundTrip(t *testing.T) {
	originalEvents := []*models.Event{
		{
			ID:          "event-1",
			UserID:      "user123",
			Title:       "Round Trip Test",
			Description: "Testing round trip",
			Location:    "Test Location",
			StartTime:   time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			EndTime:     time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
		},
	}

	// Generate CSV
	csvData, err := GenerateCSV(originalEvents)
	if err != nil {
		t.Fatalf("GenerateCSV failed: %v", err)
	}

	// Parse it back
	parsedEvents, err := ParseCSV(strings.NewReader(string(csvData)), "user456")
	if err != nil {
		t.Fatalf("ParseCSV failed: %v", err)
	}

	if len(parsedEvents) != 1 {
		t.Fatalf("Expected 1 event after round trip, got %d", len(parsedEvents))
	}

	// Verify content matches (ignoring IDs which are regenerated)
	if parsedEvents[0].Title != originalEvents[0].Title { //nolint:gosec
		t.Errorf("Title mismatch: expected '%s', got '%s'", originalEvents[0].Title, parsedEvents[0].Title)
	}
	if parsedEvents[0].Description != originalEvents[0].Description {
		t.Errorf("Description mismatch: expected '%s', got '%s'", originalEvents[0].Description, parsedEvents[0].Description)
	}
	if parsedEvents[0].Location != originalEvents[0].Location {
		t.Errorf("Location mismatch: expected '%s', got '%s'", originalEvents[0].Location, parsedEvents[0].Location)
	}

	// Note: Time comparison needs to account for the format used in CSV
	if parsedEvents[0].StartTime.Year() != originalEvents[0].StartTime.Year() ||
		parsedEvents[0].StartTime.Month() != originalEvents[0].StartTime.Month() ||
		parsedEvents[0].StartTime.Day() != originalEvents[0].StartTime.Day() ||
		parsedEvents[0].StartTime.Hour() != originalEvents[0].StartTime.Hour() ||
		parsedEvents[0].StartTime.Minute() != originalEvents[0].StartTime.Minute() {
		t.Errorf("StartTime mismatch: expected %v, got %v", originalEvents[0].StartTime, parsedEvents[0].StartTime)
	}
}
