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

// parseStartEnd returns start and end times for the event. DATE-only properties are parsed as UTC midnight.
func parseStartEnd(vEvent *ics.VEvent) (time.Time, time.Time) {
	var start time.Time
	var end time.Time

	// Start
	propStart := vEvent.GetProperty(ics.ComponentPropertyDtStart)
	if s, err := vEvent.GetStartAt(); err == nil && !s.IsZero() {
		start = s.UTC()
		// If it's floating time (no Z and no TZID), the library parses in local time.
		// We want to treat floating time as UTC to avoid system-local dependency.
		if propStart != nil && !strings.HasSuffix(propStart.Value, "Z") {
			if _, hasTZID := propStart.ICalParameters["TZID"]; !hasTZID {
				for _, layout := range []string{"20060102T150405", "20060102"} {
					if t, err := time.Parse(layout, propStart.Value); err == nil {
						start = t.UTC()
						break
					}
				}
			}
		}
	} else if propStart != nil {
		// Fallback manual parsing if GetStartAt failed but we have a property
		for _, layout := range []string{"20060102T150405Z", "20060102T150405", "20060102"} {
			if t, err := time.Parse(layout, propStart.Value); err == nil {
				start = t.UTC()
				break
			}
		}
	}

	// End
	propEnd := vEvent.GetProperty(ics.ComponentPropertyDtEnd)
	if e, err := vEvent.GetEndAt(); err == nil && !e.IsZero() {
		end = e.UTC()
		// Handle floating time for End
		if propEnd != nil && !strings.HasSuffix(propEnd.Value, "Z") {
			if _, hasTZID := propEnd.ICalParameters["TZID"]; !hasTZID {
				for _, layout := range []string{"20060102T150405", "20060102"} {
					if t, err := time.Parse(layout, propEnd.Value); err == nil {
						end = t.UTC()
						break
					}
				}
			}
		}
	} else if propEnd != nil {
		for _, layout := range []string{"20060102T150405Z", "20060102T150405", "20060102"} {
			if t, err := time.Parse(layout, propEnd.Value); err == nil {
				end = t.UTC()
				break
			}
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
