package formats

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/waydxd/Orbit-core/internal/shared/models"
)

const (
	icsDateFormat     = "20060102T150405Z"
	icsDateOnlyFormat = "20060102"
)

// ParseICS parses an ICS file and returns a slice of events.
// It supports VEVENT components with standard properties.
func ParseICS(r io.Reader, userID string) ([]*models.Event, error) {
	var events []*models.Event
	scanner := bufio.NewScanner(r)

	var currentEvent *models.Event
	var inEvent bool
	var propertyBuffer strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// Handle line folding (lines starting with space or tab are continuations)
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			propertyBuffer.WriteString(strings.TrimLeft(line, " \t"))
			continue
		}

		// Process the previous property if we have one
		if propertyBuffer.Len() > 0 {
			if inEvent && currentEvent != nil {
				parseICSProperty(propertyBuffer.String(), currentEvent)
			}
			propertyBuffer.Reset()
		}

		// Start buffering the new property
		propertyBuffer.WriteString(line)

		// Check for BEGIN/END markers
		upperLine := strings.ToUpper(line)
		switch upperLine {
		case "BEGIN:VEVENT":
			inEvent = true
			currentEvent = &models.Event{
				ID:        uuid.New().String(),
				UserID:    userID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
		case "END:VEVENT":
			if currentEvent != nil {
				// Process the last property before END:VEVENT
				if propertyBuffer.Len() > 0 && !strings.HasPrefix(strings.ToUpper(propertyBuffer.String()), "END:") {
					parseICSProperty(propertyBuffer.String(), currentEvent)
				}
				events = append(events, currentEvent)
			}
			inEvent = false
			currentEvent = nil
			propertyBuffer.Reset()
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read ICS file: %w", err)
	}

	return events, nil
}

// parseICSProperty parses a single ICS property line and updates the event
func parseICSProperty(line string, event *models.Event) {
	// Split property name and value
	colonIdx := strings.Index(line, ":")
	if colonIdx == -1 {
		return
	}

	propertyPart := line[:colonIdx]
	value := line[colonIdx+1:]

	// Extract property name (before any parameters)
	semicolonIdx := strings.Index(propertyPart, ";")
	var propertyName string
	if semicolonIdx == -1 {
		propertyName = propertyPart
	} else {
		propertyName = propertyPart[:semicolonIdx]
	}

	propertyName = strings.ToUpper(propertyName)

	switch propertyName {
	case "SUMMARY":
		event.Title = unescapeICSValue(value)
	case "DESCRIPTION":
		event.Description = unescapeICSValue(value)
	case "LOCATION":
		event.Location = unescapeICSValue(value)
	case "DTSTART":
		if t, err := parseICSDateTime(value, propertyPart); err == nil {
			event.StartTime = t
		}
	case "DTEND":
		if t, err := parseICSDateTime(value, propertyPart); err == nil {
			event.EndTime = t
		}
	case "UID":
		// Optionally use UID if needed for external tracking
		// event.ExternalID = value
	}
}

// parseICSDateTime parses an ICS date-time value
func parseICSDateTime(value, propertyPart string) (time.Time, error) {
	value = strings.TrimSpace(value)

	// Check for VALUE=DATE parameter (date-only format)
	if strings.Contains(strings.ToUpper(propertyPart), "VALUE=DATE") {
		return time.Parse(icsDateOnlyFormat, value)
	}

	// Check for TZID parameter
	if strings.Contains(strings.ToUpper(propertyPart), "TZID=") {
		// Extract timezone
		params := strings.Split(propertyPart, ";")
		for _, param := range params {
			if strings.HasPrefix(strings.ToUpper(param), "TZID=") {
				tzName := strings.TrimPrefix(param, "TZID=")
				tzName = strings.TrimPrefix(tzName, "tzid=")
				if loc, err := time.LoadLocation(tzName); err == nil {
					// Parse as local time in the specified timezone
					localFormat := "20060102T150405"
					t, err := time.ParseInLocation(localFormat, value, loc)
					if err == nil {
						return t, nil
					}
				}
			}
		}
	}

	// Try UTC format (with Z suffix)
	if strings.HasSuffix(value, "Z") {
		return time.Parse(icsDateFormat, value)
	}

	// Try local time format
	localFormat := "20060102T150405"
	return time.Parse(localFormat, value)
}

// unescapeICSValue unescapes ICS property values
func unescapeICSValue(value string) string {
	value = strings.ReplaceAll(value, "\\n", "\n")
	value = strings.ReplaceAll(value, "\\N", "\n")
	value = strings.ReplaceAll(value, "\\,", ",")
	value = strings.ReplaceAll(value, "\\;", ";")
	value = strings.ReplaceAll(value, "\\\\", "\\")
	return value
}

// GenerateICS generates an ICS file from a slice of events
func GenerateICS(events []*models.Event) ([]byte, error) {
	var buf bytes.Buffer

	// Write calendar header
	buf.WriteString("BEGIN:VCALENDAR\r\n")
	buf.WriteString("VERSION:2.0\r\n")
	buf.WriteString("PRODID:-//Orbit-core//Calendar//EN\r\n")
	buf.WriteString("CALSCALE:GREGORIAN\r\n")
	buf.WriteString("METHOD:PUBLISH\r\n")

	// Write events
	for _, event := range events {
		buf.WriteString("BEGIN:VEVENT\r\n")

		// UID
		buf.WriteString(fmt.Sprintf("UID:%s@orbit-core\r\n", event.ID))

		// Timestamps
		buf.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", time.Now().UTC().Format(icsDateFormat)))
		buf.WriteString(fmt.Sprintf("DTSTART:%s\r\n", event.StartTime.UTC().Format(icsDateFormat)))
		buf.WriteString(fmt.Sprintf("DTEND:%s\r\n", event.EndTime.UTC().Format(icsDateFormat)))

		// Summary (title)
		if event.Title != "" {
			buf.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", escapeICSValue(event.Title)))
		}

		// Description
		if event.Description != "" {
			buf.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", escapeICSValue(event.Description)))
		}

		// Location
		if event.Location != "" {
			buf.WriteString(fmt.Sprintf("LOCATION:%s\r\n", escapeICSValue(event.Location)))
		}

		// Created/Last-Modified
		buf.WriteString(fmt.Sprintf("CREATED:%s\r\n", event.CreatedAt.UTC().Format(icsDateFormat)))
		buf.WriteString(fmt.Sprintf("LAST-MODIFIED:%s\r\n", event.UpdatedAt.UTC().Format(icsDateFormat)))

		buf.WriteString("END:VEVENT\r\n")
	}

	buf.WriteString("END:VCALENDAR\r\n")

	return buf.Bytes(), nil
}

// escapeICSValue escapes special characters in ICS property values
func escapeICSValue(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, ";", "\\;")
	value = strings.ReplaceAll(value, ",", "\\,")
	value = strings.ReplaceAll(value, "\n", "\\n")
	return value
}
