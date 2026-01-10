package formats

import (
	"strings"
	"testing"
	"time"

	"github.com/waydxd/Orbit-core/internal/shared/models"
)

func TestParseICS(t *testing.T) {
	icsContent := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:test-event-1@test.com
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Test Meeting
DESCRIPTION:This is a test meeting description
LOCATION:Conference Room A
END:VEVENT
BEGIN:VEVENT
UID:test-event-2@test.com
DTSTART:20240116T140000Z
DTEND:20240116T150000Z
SUMMARY:Another Meeting
END:VEVENT
END:VCALENDAR`

	events, err := ParseICS(strings.NewReader(icsContent), "user123")
	if err != nil {
		t.Fatalf("ParseICS failed: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}

	// Check first event
	if events[0].Title != "Test Meeting" {
		t.Errorf("Expected title 'Test Meeting', got '%s'", events[0].Title)
	}
	if events[0].Description != "This is a test meeting description" {
		t.Errorf("Expected description 'This is a test meeting description', got '%s'", events[0].Description)
	}
	if events[0].Location != "Conference Room A" {
		t.Errorf("Expected location 'Conference Room A', got '%s'", events[0].Location)
	}
	if events[0].UserID != "user123" {
		t.Errorf("Expected userID 'user123', got '%s'", events[0].UserID)
	}

	// Check date parsing
	expectedStart := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	if !events[0].StartTime.Equal(expectedStart) {
		t.Errorf("Expected start time %v, got %v", expectedStart, events[0].StartTime)
	}

	// Check second event
	if events[1].Title != "Another Meeting" {
		t.Errorf("Expected title 'Another Meeting', got '%s'", events[1].Title)
	}
}

func TestParseICSWithEscapedCharacters(t *testing.T) {
	icsContent := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test@test.com
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Meeting with John\, Jane\, and Bob
DESCRIPTION:Line 1\nLine 2\nLine 3
END:VEVENT
END:VCALENDAR`

	events, err := ParseICS(strings.NewReader(icsContent), "user123")
	if err != nil {
		t.Fatalf("ParseICS failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].Title != "Meeting with John, Jane, and Bob" {
		t.Errorf("Expected unescaped title, got '%s'", events[0].Title)
	}
	if events[0].Description != "Line 1\nLine 2\nLine 3" {
		t.Errorf("Expected unescaped description with newlines, got '%s'", events[0].Description)
	}
}

func TestParseICSDateOnly(t *testing.T) {
	icsContent := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:allday@test.com
DTSTART;VALUE=DATE:20240115
DTEND;VALUE=DATE:20240116
SUMMARY:All Day Event
END:VEVENT
END:VCALENDAR`

	events, err := ParseICS(strings.NewReader(icsContent), "user123")
	if err != nil {
		t.Fatalf("ParseICS failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	expectedStart := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	if !events[0].StartTime.Equal(expectedStart) {
		t.Errorf("Expected start time %v, got %v", expectedStart, events[0].StartTime)
	}
}

func TestGenerateICS(t *testing.T) {
	events := []*models.Event{
		{
			ID:          "event-1",
			UserID:      "user123",
			Title:       "Test Event",
			Description: "Test description",
			Location:    "Room 101",
			StartTime:   time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			EndTime:     time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
			CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	icsData, err := GenerateICS(events)
	if err != nil {
		t.Fatalf("GenerateICS failed: %v", err)
	}

	icsString := string(icsData)

	// Check required components
	if !strings.Contains(icsString, "BEGIN:VCALENDAR") {
		t.Error("Missing BEGIN:VCALENDAR")
	}
	if !strings.Contains(icsString, "END:VCALENDAR") {
		t.Error("Missing END:VCALENDAR")
	}
	if !strings.Contains(icsString, "BEGIN:VEVENT") {
		t.Error("Missing BEGIN:VEVENT")
	}
	if !strings.Contains(icsString, "END:VEVENT") {
		t.Error("Missing END:VEVENT")
	}
	if !strings.Contains(icsString, "SUMMARY:Test Event") {
		t.Error("Missing SUMMARY")
	}
	if !strings.Contains(icsString, "DESCRIPTION:Test description") {
		t.Error("Missing DESCRIPTION")
	}
	if !strings.Contains(icsString, "LOCATION:Room 101") {
		t.Error("Missing LOCATION")
	}
}

func TestGenerateICSWithSpecialCharacters(t *testing.T) {
	events := []*models.Event{
		{
			ID:          "event-1",
			Title:       "Meeting with John, Jane",
			Description: "Line 1\nLine 2",
			StartTime:   time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			EndTime:     time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	icsData, err := GenerateICS(events)
	if err != nil {
		t.Fatalf("GenerateICS failed: %v", err)
	}

	icsString := string(icsData)

	// Check escaped characters
	if !strings.Contains(icsString, "Meeting with John\\, Jane") {
		t.Error("Comma not properly escaped in title")
	}
	if !strings.Contains(icsString, "Line 1\\nLine 2") {
		t.Error("Newline not properly escaped in description")
	}
}

func TestICSRoundTrip(t *testing.T) {
	originalEvents := []*models.Event{
		{
			ID:          "event-1",
			UserID:      "user123",
			Title:       "Round Trip Test",
			Description: "Testing round trip",
			Location:    "Test Location",
			StartTime:   time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			EndTime:     time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	// Generate ICS
	icsData, err := GenerateICS(originalEvents)
	if err != nil {
		t.Fatalf("GenerateICS failed: %v", err)
	}

	// Parse it back
	parsedEvents, err := ParseICS(strings.NewReader(string(icsData)), "user456")
	if err != nil {
		t.Fatalf("ParseICS failed: %v", err)
	}

	if len(parsedEvents) != 1 {
		t.Fatalf("Expected 1 event after round trip, got %d", len(parsedEvents))
	}

	// Verify content matches (ignoring IDs which are regenerated)
	if parsedEvents[0].Title != originalEvents[0].Title {
		t.Errorf("Title mismatch: expected '%s', got '%s'", originalEvents[0].Title, parsedEvents[0].Title)
	}
	if parsedEvents[0].Description != originalEvents[0].Description {
		t.Errorf("Description mismatch: expected '%s', got '%s'", originalEvents[0].Description, parsedEvents[0].Description)
	}
	if parsedEvents[0].Location != originalEvents[0].Location {
		t.Errorf("Location mismatch: expected '%s', got '%s'", originalEvents[0].Location, parsedEvents[0].Location)
	}
	if !parsedEvents[0].StartTime.Equal(originalEvents[0].StartTime) {
		t.Errorf("StartTime mismatch: expected %v, got %v", originalEvents[0].StartTime, parsedEvents[0].StartTime)
	}
	if !parsedEvents[0].EndTime.Equal(originalEvents[0].EndTime) {
		t.Errorf("EndTime mismatch: expected %v, got %v", originalEvents[0].EndTime, parsedEvents[0].EndTime)
	}
}
