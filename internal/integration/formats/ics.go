package formats

import (
	"fmt"
	"io"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/google/uuid"
	"github.com/waydxd/Orbit-core/internal/shared/models"
)

// ParseICS parses an ICS file and returns a slice of events using github.com/arran4/golang-ical.
func ParseICS(r io.Reader, userID string) ([]*models.Event, error) {
	cal, err := ics.ParseCalendar(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ICS: %w", err)
	}

	var events []*models.Event
	for _, vEvent := range cal.Events() {
		events = append(events, vEventToEvent(vEvent, userID))
	}

	return events, nil
}

// vEventToEvent converts a library VEvent into our models.Event.
func vEventToEvent(vEvent *ics.VEvent, userID string) *models.Event {
	event := &models.Event{
		ID:        vEvent.Id(),
		UserID:    userID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Basic string properties
	event.Title = getPropValue(vEvent, ics.ComponentPropertySummary)
	event.Description = getPropValue(vEvent, ics.ComponentPropertyDescription)
	event.Location = getPropValue(vEvent, ics.ComponentPropertyLocation)

	// Parse start/end times (handles DATE-only and full datetimes)
	start, end := parseStartEnd(vEvent)
	if !start.IsZero() {
		event.StartTime = start
	}
	if !end.IsZero() {
		event.EndTime = end
	}

	// Ensure we have a valid ID
	if event.ID == "" {
		event.ID = uuid.New().String()
	}

	return event
}

// getPropValue safely retrieves the IANA property value for a component property.
func getPropValue(vEvent *ics.VEvent, prop ics.ComponentProperty) string {
	p := vEvent.GetProperty(prop)
	if p == nil {
		return ""
	}
	return p.Value
}

// parseStartEnd returns start and end times for the event. DATE-only properties are parsed as UTC midnight.
func parseStartEnd(vEvent *ics.VEvent) (time.Time, time.Time) {
	var start time.Time
	var end time.Time

	// Start
	if prop := vEvent.GetProperty(ics.ComponentPropertyDtStart); prop != nil {
		if prop.GetValueType() == ics.ValueDataTypeDate {
			if t, err := time.Parse("20060102", prop.Value); err == nil {
				start = t.UTC()
			}
		}
	}
	if start.IsZero() {
		if s, err := vEvent.GetStartAt(); err == nil {
			start = s.UTC()
		}
	}

	// End
	if prop := vEvent.GetProperty(ics.ComponentPropertyDtEnd); prop != nil {
		if prop.GetValueType() == ics.ValueDataTypeDate {
			if t, err := time.Parse("20060102", prop.Value); err == nil {
				end = t.UTC()
			}
		}
	}
	if end.IsZero() {
		if e, err := vEvent.GetEndAt(); err == nil {
			end = e.UTC()
		}
	}

	return start, end
}

// GenerateICS generates an ICS file from a slice of events using github.com/arran4/golang-ical.
func GenerateICS(events []*models.Event) ([]byte, error) {
	cal := ics.NewCalendar()
	cal.SetMethod(ics.MethodPublish)
	cal.SetProductId("-//Orbit-core//Calendar//EN")

	for _, e := range events {
		id := e.ID
		if id == "" {
			id = uuid.New().String()
		}
		event := cal.AddEvent(id)

		event.SetSummary(e.Title)
		event.SetDescription(e.Description)
		event.SetLocation(e.Location)
		event.SetStartAt(e.StartTime)
		event.SetEndAt(e.EndTime)
		event.SetCreatedTime(e.CreatedAt)
		event.SetModifiedAt(e.UpdatedAt)
		event.SetDtStampTime(time.Now())
	}

	return []byte(cal.Serialize()), nil
}
