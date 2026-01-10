package formats

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/waydxd/Orbit-core/internal/shared/models"
)

// CSV column headers for calendar events
const (
	csvDateTimeFormat = "2006-01-02 15:04:05"
)

var csvHeaders = []string{
	"Title",
	"Description",
	"Start Time",
	"End Time",
	"Location",
}

// ParseCSV parses a CSV file and returns a slice of events.
// Expected columns: Title, Description, Start Time, End Time, Location
func ParseCSV(r io.Reader, userID string) ([]*models.Event, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // Allow variable number of fields
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) == 0 {
		return []*models.Event{}, nil
	}

	// Parse header row to determine column indices
	headerRow := records[0]
	colIndex := mapCSVHeaders(headerRow)

	var events []*models.Event

	// Parse data rows (skip header)
	for i := 1; i < len(records); i++ {
		row := records[i]
		if len(row) == 0 {
			continue
		}

		event := &models.Event{
			ID:        uuid.New().String(),
			UserID:    userID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Extract values based on column mapping
		if idx, ok := colIndex["title"]; ok && idx < len(row) {
			event.Title = strings.TrimSpace(row[idx])
		}
		if idx, ok := colIndex["description"]; ok && idx < len(row) {
			event.Description = strings.TrimSpace(row[idx])
		}
		if idx, ok := colIndex["start_time"]; ok && idx < len(row) {
			if t, err := parseCSVDateTime(row[idx]); err == nil {
				event.StartTime = t
			}
		}
		if idx, ok := colIndex["end_time"]; ok && idx < len(row) {
			if t, err := parseCSVDateTime(row[idx]); err == nil {
				event.EndTime = t
			}
		}
		if idx, ok := colIndex["location"]; ok && idx < len(row) {
			event.Location = strings.TrimSpace(row[idx])
		}

		// Only add events with at least a title
		if event.Title != "" {
			events = append(events, event)
		}
	}

	return events, nil
}

// mapCSVHeaders creates a mapping from normalized header names to column indices
func mapCSVHeaders(headers []string) map[string]int {
	colIndex := make(map[string]int)

	for i, header := range headers {
		normalized := normalizeCSVHeader(header)
		switch normalized {
		case "title", "subject", "summary", "name", "event_title", "event title":
			colIndex["title"] = i
		case "description", "desc", "details", "notes", "body":
			colIndex["description"] = i
		case "start", "start_time", "start time", "start_date", "start date", "begins", "dtstart":
			colIndex["start_time"] = i
		case "end", "end_time", "end time", "end_date", "end date", "ends", "dtend":
			colIndex["end_time"] = i
		case "location", "place", "where", "venue":
			colIndex["location"] = i
		}
	}

	return colIndex
}

// normalizeCSVHeader normalizes a CSV header for comparison
func normalizeCSVHeader(header string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	header = strings.ReplaceAll(header, "_", " ")
	header = strings.ReplaceAll(header, "-", " ")
	return header
}

// parseCSVDateTime parses a date-time string in various formats
func parseCSVDateTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty date-time value")
	}

	// List of supported date-time formats
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"01/02/2006 15:04:05",
		"01/02/2006 15:04",
		"01/02/2006",
		"02/01/2006 15:04:05",
		"02/01/2006 15:04",
		"02/01/2006",
		"Jan 2, 2006 15:04:05",
		"Jan 2, 2006 15:04",
		"Jan 2, 2006",
		"2 Jan 2006 15:04:05",
		"2 Jan 2006 15:04",
		"2 Jan 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, value); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date-time: %s", value)
}

// GenerateCSV generates a CSV file from a slice of events
func GenerateCSV(events []*models.Event) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header row
	if err := writer.Write(csvHeaders); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write event rows
	for _, event := range events {
		row := []string{
			event.Title,
			event.Description,
			event.StartTime.Format(csvDateTimeFormat),
			event.EndTime.Format(csvDateTimeFormat),
			event.Location,
		}
		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("failed to flush CSV writer: %w", err)
	}

	return buf.Bytes(), nil
}
