package formats

import (
	"fmt"
	"io"
	"strings"
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
	uid := vEvent.Id()
	recurrenceID := getPropValue(vEvent, ics.ComponentPropertyRecurrenceId)

	id := uid
	if recurrenceID != "" {
		id = uid + "_" + recurrenceID
	}

	// Ensure we have a valid UUID for our database.
	// If the ICS UID is not a valid UUID (or if it's a combination with recurrence ID),
	// generate a deterministic one from it to maintain idempotency.
	if _, err := uuid.Parse(id); err != nil {
		if id != "" {
			id = uuid.NewSHA1(uuid.NameSpaceDNS, []byte(id)).String()
		} else {
			id = uuid.New().String()
		}
	}

	event := &models.Event{
		ID:        id,
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

// parseICalTime centralizes parsing logic used for DTSTART/DTEND handling.
// It attempts to use the library getter (GetStartAt/GetEndAt) and falls back to
// parsing the raw property value. It also handles "floating" times (no trailing Z
// and no TZID) by interpreting them as UTC.
func parseICalTime(vEvent *ics.VEvent, propType ics.ComponentProperty, getter func(*ics.VEvent) (time.Time, error)) time.Time {
	prop := vEvent.GetProperty(propType)

	// Prefer the library's parsed value when available
	if t, err := getter(vEvent); err == nil && !t.IsZero() {
		t = t.UTC()
		// If it's floating time (no Z and no TZID), treat as UTC
		if prop != nil && !strings.HasSuffix(prop.Value, "Z") {
			if _, hasTZID := prop.ICalParameters["TZID"]; !hasTZID {
				for _, layout := range []string{"20060102T150405", "20060102"} {
					if parsed, err := time.Parse(layout, prop.Value); err == nil {
						return parsed.UTC()
					}
				}
			}
		}
		return t
	}

	// Fallback: parse raw property value if library parsing not available
	if prop != nil {
		for _, layout := range []string{"20060102T150405Z", "20060102T150405", "20060102"} {
			if parsed, err := time.Parse(layout, prop.Value); err == nil {
				return parsed.UTC()
			}
		}
	}

	return time.Time{}
}

// parseStartEnd returns start and end times for the event. DATE-only properties are parsed as UTC midnight.
func parseStartEnd(vEvent *ics.VEvent) (time.Time, time.Time) {
	start := parseICalTime(vEvent, ics.ComponentPropertyDtStart, func(v *ics.VEvent) (time.Time, error) { return v.GetStartAt() })
	end := parseICalTime(vEvent, ics.ComponentPropertyDtEnd, func(v *ics.VEvent) (time.Time, error) { return v.GetEndAt() })
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

		if e.IsRecurring && e.RecurrenceRule != "" {
			event.SetProperty(ics.ComponentPropertyRrule, e.RecurrenceRule)
		}
		if e.RecurrenceException != "" {
			// Split by newline if we stored multiple exceptions
			for _, ex := range strings.Split(e.RecurrenceException, "\n") {
				if strings.TrimSpace(ex) != "" {
					event.AddProperty(ics.ComponentPropertyExdate, ex)
				}
			}
		}
	}

	return []byte(cal.Serialize()), nil
}
