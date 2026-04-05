package calendar

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	pb "github.com/waydxd/Orbit-core/proto/calendar"
)

// ===== gRPC Server Implementation =====

// GetCalendarData implements CalendarDataService.GetCalendarData
func (s *Service) GetCalendarData(ctx context.Context, req *pb.GetCalendarDataRequest) (*pb.GetCalendarDataResponse, error) {
	s.logger.Info("GetCalendarData called by Agent", "user_id", req.UserId)

	startTime := time.Unix(req.StartTime, 0)
	endTime := time.Unix(req.EndTime, 0)

	events, err := s.eventRepo.ListEvents(ctx, req.UserId, startTime, endTime)
	if err != nil {
		s.logger.Error("failed to get calendar data", "err", err)
		return &pb.GetCalendarDataResponse{
			Success: false,
			Message: fmt.Sprintf("failed to get calendar data: %v", err),
		}, nil
	}

	pbEvents := make([]*pb.Event, len(events))
	for i, event := range events {
		pbEvents[i] = &pb.Event{
			Id:          event.ID,
			Title:       event.Title,
			Description: event.Description,
			StartTime:   event.StartTime.Unix(),
			EndTime:     event.EndTime.Unix(),
			Location:    event.Location,
		}
	}

	return &pb.GetCalendarDataResponse{
		Events:  pbEvents,
		Success: true,
		Message: "Calendar data retrieved successfully",
	}, nil
}

// GetUserAvailability implements CalendarDataService.GetUserAvailability
func (s *Service) GetUserAvailability(ctx context.Context, req *pb.GetUserAvailabilityRequest) (*pb.GetUserAvailabilityResponse, error) {
	s.logger.Info("GetUserAvailability called by Agent", "user_id", req.UserId)

	startTime := time.Unix(req.StartTime, 0)
	endTime := time.Unix(req.EndTime, 0)

	events, err := s.eventRepo.ListEvents(ctx, req.UserId, startTime, endTime)
	if err != nil {
		s.logger.Error("failed to check availability", "err", err)
		return &pb.GetUserAvailabilityResponse{
			Success: false,
			Message: fmt.Sprintf("failed to check availability: %v", err),
		}, nil
	}

	available := len(events) == 0
	reason := "No conflicts found"
	if !available {
		reason = fmt.Sprintf("%d conflicting events found", len(events))
	}

	pbEvents := make([]*pb.Event, len(events))
	for i, event := range events {
		pbEvents[i] = &pb.Event{
			Id:          event.ID,
			Title:       event.Title,
			Description: event.Description,
			StartTime:   event.StartTime.Unix(),
			EndTime:     event.EndTime.Unix(),
			Location:    event.Location,
		}
	}

	status := &pb.AvailabilityStatus{
		Available:         available,
		Reason:            reason,
		ConflictingEvents: pbEvents,
	}

	return &pb.GetUserAvailabilityResponse{
		Status:  status,
		Success: true,
		Message: "Availability check completed",
	}, nil
}

// QueryEvents implements CalendarDataService.QueryEvents
func (s *Service) QueryEvents(ctx context.Context, req *pb.QueryEventsRequest) (*pb.QueryEventsResponse, error) {
	s.logger.Info("QueryEvents called by Agent", "user_id", req.UserId)

	startTime := time.Unix(req.StartTime, 0)
	endTime := time.Unix(req.EndTime, 0)

	events, err := s.eventRepo.ListEvents(ctx, req.UserId, startTime, endTime)
	if err != nil {
		s.logger.Error("failed to query events", "err", err)
		return &pb.QueryEventsResponse{
			Success: false,
			Message: fmt.Sprintf("failed to query events: %v", err),
		}, nil
	}

	pbEvents := make([]*pb.Event, len(events))
	for i, event := range events {
		pbEvents[i] = &pb.Event{
			Id:          event.ID,
			Title:       event.Title,
			Description: event.Description,
			StartTime:   event.StartTime.Unix(),
			EndTime:     event.EndTime.Unix(),
			Location:    event.Location,
		}
	}

	// Safely convert length to int32 avoiding overflow
	var totalCount int32
	if len(pbEvents) > int(^uint32(0)>>1) { // larger than max int32
		totalCount = -1 // indicate overflow
	} else {
		// #nosec G115 -- len(pbEvents) is guaranteed to be within int32 range
		totalCount = int32(len(pbEvents))
	}

	return &pb.QueryEventsResponse{
		Events:     pbEvents,
		TotalCount: totalCount,
		Success:    true,
		Message:    "Query executed successfully",
	}, nil
}

// Adapter methods to satisfy agent.CalendarServiceInterface and gateway.CalendarServiceInterface

// ListEventsAdapter returns events across users (userID omitted) or can be extended to filter by status.
func (s *Service) ListEventsAdapter(ctx context.Context, startTime, endTime int64, status string) ([]interface{}, error) {
	_ = status // mark as used until filtering is implemented
	st := time.Unix(startTime, 0)
	en := time.Unix(endTime, 0)

	events, err := s.eventRepo.ListEvents(ctx, "", st, en)
	if err != nil {
		s.logger.Error("failed to list events (adapter)", "err", err)
		return nil, err
	}

	out := make([]interface{}, len(events))
	for i, e := range events {
		out[i] = e
	}
	return out, nil
}

// CreateEventAdapter accepts flexible payloads (map[string]interface{}, *models.Event, pb.Event) and creates an event
func (s *Service) CreateEventAdapter(ctx context.Context, event interface{}) (interface{}, error) {
	// Look up timezone from user if available, default to HKT
	// First extract userID to get timezone
	var timezone string
	if v, ok := event.(map[string]interface{}); ok {
		if tz, ok := v["timezone"].(string); ok {
			timezone = tz
		}
	}
	if timezone == "" {
		timezone = "Asia/Hong_Kong" // Default to HKT
	}

	// Delegate parsing to a helper to keep cyclomatic complexity low
	ev, err := parseEventPayload(event, timezone)
	if err != nil {
		s.logger.Error("failed to parse event payload (adapter)", "err", err)
		return nil, err
	}

	if ev.ID == "" {
		ev.ID = uuid.New().String()
	}
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now()
	}
	ev.UpdatedAt = time.Now()

	if err := s.eventRepo.CreateEvent(ctx, &ev); err != nil {
		s.logger.Error("failed to create event (adapter)", "err", err)
		return nil, err
	}

	// Track event for habit detection (async, don't block response)
	if s.habitTracker != nil && !ev.IsRecurring && ev.RecurrenceRule == "" {
		trackParentCtx := context.WithoutCancel(ctx)
		go func() {
			trackCtx, trackCancel := context.WithTimeout(trackParentCtx, 5*time.Second)
			defer trackCancel()
			if err := s.habitTracker.TrackEventCreation(trackCtx, &ev); err != nil {
				s.logger.Error("failed to track event for habit detection (adapter)", "err", err)
			}
		}()
	}

	return &ev, nil
}

// parseEventPayload converts supported input types into a models.Event value.
// Supported input types: map[string]interface{}, *models.Event, models.Event, *pb.Event
func parseEventPayload(event interface{}, timezone string) (models.Event, error) {
	var ev models.Event
	switch v := event.(type) {
	case map[string]interface{}:
		if id, ok := v["user_id"].(string); ok {
			ev.UserID = id
		}
		if title, ok := v["title"].(string); ok {
			ev.Title = title
		}
		if desc, ok := v["description"].(string); ok {
			ev.Description = desc
		}
		if loc, ok := v["location"].(string); ok {
			ev.Location = loc
		}
		// Parse start_time and end_time using helper
		if st, ok := v["start_time"]; ok {
			if t, err := parseTimeFromInterface(st, timezone); err == nil {
				ev.StartTime = t
			}
		}
		if et, ok := v["end_time"]; ok {
			if t, err := parseTimeFromInterface(et, timezone); err == nil {
				ev.EndTime = t
			}
		}
	case *models.Event:
		ev = *v
	case models.Event:
		ev = v
	case *pb.Event:
		ev.ID = v.Id
		ev.Title = v.Title
		ev.Description = v.Description
		ev.StartTime = time.Unix(v.StartTime, 0)
		ev.EndTime = time.Unix(v.EndTime, 0)
		ev.Location = v.Location
	default:
		return models.Event{}, fmt.Errorf("unsupported event type")
	}
	return ev, nil
}

// parseTimeFromInterface parses time from various interface{} types (int64, float64, string)
func parseTimeFromInterface(v interface{}, timezone string) (time.Time, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil || timezone == "" {
		loc, _ = time.LoadLocation("Asia/Hong_Kong")
	}

	switch tv := v.(type) {
	case int64:
		return time.Unix(tv, 0).In(loc), nil
	case float64:
		return time.Unix(int64(tv), 0).In(loc), nil
	case string:
		t, err := time.Parse(time.RFC3339, tv)
		if err == nil {
			return t.In(loc), nil
		}
		// Try parsing without timezone
		t, err = time.ParseInLocation("2006-01-02T15:04:05", tv, loc)
		if err == nil {
			return t, nil
		}
		t, err = time.ParseInLocation("2006-01-02T15:04:05.000Z", tv, loc)
		if err == nil {
			return t, nil
		}
		return time.Time{}, fmt.Errorf("unsupported time format: %v", err)
	default:
		return time.Time{}, fmt.Errorf("unsupported time type")
	}
}

// UpdateEventAdapter updates an event by id using flexible payloads
func (s *Service) UpdateEventAdapter(ctx context.Context, id string, event interface{}) (interface{}, error) {
	existing, err := s.eventRepo.GetEventByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get event for update", "id", id, "err", err)
		return nil, err
	}

	existing, err = applyEventUpdatePayload(existing, event)
	if err != nil {
		return nil, err
	}

	existing.UpdatedAt = time.Now()
	if err := s.eventRepo.UpdateEvent(ctx, existing); err != nil {
		s.logger.Error("failed to update event (adapter)", "err", err)
		return nil, err
	}

	if s.habitTracker != nil && !existing.IsRecurring && existing.RecurrenceRule == "" {
		trackParentCtx := context.WithoutCancel(ctx)
		go func() {
			trackCtx, trackCancel := context.WithTimeout(trackParentCtx, 5*time.Second)
			defer trackCancel()
			if err := s.habitTracker.TrackEventCreation(trackCtx, existing); err != nil {
				s.logger.Error("failed to track event for habit detection (adapter udpate)", "err", err)
			}
		}()
	}

	return existing, nil
}

func applyEventUpdatePayload(existing *models.Event, event interface{}) (*models.Event, error) {
	switch v := event.(type) {
	case map[string]interface{}:
		if title, ok := v["title"].(string); ok {
			existing.Title = title
		}
		if desc, ok := v["description"].(string); ok {
			existing.Description = desc
		}
		if loc, ok := v["location"].(string); ok {
			existing.Location = loc
		}
		if sts, ok := v["start_time"].(string); ok {
			if t, err := time.Parse(time.RFC3339, sts); err == nil {
				existing.StartTime = t
			}
		}
		if ets, ok := v["end_time"].(string); ok {
			if t, err := time.Parse(time.RFC3339, ets); err == nil {
				existing.EndTime = t
			}
		}
	case *models.Event:
		// Replace the existing pointer with provided pointer
		existing = v
	case models.Event:
		// Copy fields from value into existing pointer
		existing.Title = v.Title
		existing.Description = v.Description
		existing.StartTime = v.StartTime
		existing.EndTime = v.EndTime
		existing.Location = v.Location
	case *pb.Event:
		existing.Title = v.Title
		existing.Description = v.Description
		existing.StartTime = time.Unix(v.StartTime, 0)
		existing.EndTime = time.Unix(v.EndTime, 0)
		existing.Location = v.Location
	default:
		return nil, fmt.Errorf("unsupported event type for update")
	}

	return existing, nil
}

// DeleteEventAdapter deletes an event by id
func (s *Service) DeleteEventAdapter(ctx context.Context, id string) error {
	if err := s.eventRepo.DeleteEvent(ctx, id); err != nil {
		s.logger.Error("failed to delete event (adapter)", "id", id, "err", err)
		return err
	}
	return nil
}

// CreateEvent implements CalendarService.CreateEvent
func (s *Service) CreateEvent(ctx context.Context, req *pb.CreateEventRequest) (*pb.CreateEventResponse, error) {
	s.logger.Info("CreateEvent called via gRPC", "user_id", req.UserId)

	event := &models.Event{
		ID:          uuid.New().String(),
		UserID:      req.UserId,
		Title:       req.Title,
		Description: req.Description,
		StartTime:   time.Unix(req.StartTime, 0),
		EndTime:     time.Unix(req.EndTime, 0),
		Location:    req.Location,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.eventRepo.CreateEvent(ctx, event); err != nil {
		s.logger.Error("failed to create event via gRPC", "err", err)
		return &pb.CreateEventResponse{
			Success: false,
			Message: fmt.Sprintf("failed to create event: %v", err),
		}, nil
	}

	// Track event for habit detection (async, don't block response)
	if s.habitTracker != nil && !event.IsRecurring && event.RecurrenceRule == "" {
		trackParentCtx := context.WithoutCancel(ctx)
		go func() {
			trackCtx, trackCancel := context.WithTimeout(trackParentCtx, 5*time.Second)
			defer trackCancel()
			if err := s.habitTracker.TrackEventCreation(trackCtx, event); err != nil {
				s.logger.Error("failed to track event for habit detection (gRPC)", "err", err)
			}
		}()
	}

	pbEvent := &pb.Event{
		Id:          event.ID,
		UserId:      event.UserID,
		Title:       event.Title,
		Description: event.Description,
		StartTime:   event.StartTime.Unix(),
		EndTime:     event.EndTime.Unix(),
		Location:    event.Location,
	}

	return &pb.CreateEventResponse{
		Event:   pbEvent,
		Success: true,
		Message: "Event created successfully",
	}, nil
}

// GetEvents implements CalendarService.GetEvents
func (s *Service) GetEvents(ctx context.Context, req *pb.GetEventsRequest) (*pb.GetEventsResponse, error) {
	s.logger.Info("GetEvents called via gRPC", "user_id", req.UserId)

	startTime := time.Unix(req.StartTime, 0)
	endTime := time.Unix(req.EndTime, 0)

	events, err := s.eventRepo.ListEvents(ctx, req.UserId, startTime, endTime)
	if err != nil {
		s.logger.Error("failed to get events via gRPC", "err", err)
		return &pb.GetEventsResponse{
			Success: false,
			Message: fmt.Sprintf("failed to get events: %v", err),
		}, nil
	}

	pbEvents := make([]*pb.Event, len(events))
	for i, event := range events {
		pbEvents[i] = &pb.Event{
			Id:          event.ID,
			UserId:      event.UserID,
			Title:       event.Title,
			Description: event.Description,
			StartTime:   event.StartTime.Unix(),
			EndTime:     event.EndTime.Unix(),
			Location:    event.Location,
		}
	}

	return &pb.GetEventsResponse{
		Events:  pbEvents,
		Success: true,
		Message: "Events retrieved successfully",
	}, nil
}

// UpdateEvent implements CalendarService.UpdateEvent
func (s *Service) UpdateEvent(ctx context.Context, req *pb.UpdateEventRequest) (*pb.UpdateEventResponse, error) {
	s.logger.Info("UpdateEvent called via gRPC", "user_id", req.UserId, "event_id", req.Id)

	event, err := s.eventRepo.GetEventByID(ctx, req.Id)
	if err != nil {
		return &pb.UpdateEventResponse{
			Success: false,
			Message: fmt.Sprintf("event not found: %v", err),
		}, nil
	}

	if req.Title != "" {
		event.Title = req.Title
	}
	if req.Description != "" {
		event.Description = req.Description
	}
	if req.Location != "" {
		event.Location = req.Location
	}
	if req.StartTime != 0 {
		event.StartTime = time.Unix(req.StartTime, 0)
	}
	if req.EndTime != 0 {
		event.EndTime = time.Unix(req.EndTime, 0)
	}
	event.UpdatedAt = time.Now()

	if err := s.eventRepo.UpdateEvent(ctx, event); err != nil {
		s.logger.Error("failed to update event via gRPC", "err", err)
		return &pb.UpdateEventResponse{
			Success: false,
			Message: fmt.Sprintf("failed to update event: %v", err),
		}, nil
	}

	if s.habitTracker != nil && !event.IsRecurring && event.RecurrenceRule == "" {
		trackParentCtx := context.WithoutCancel(ctx)
		go func() {
			trackCtx, trackCancel := context.WithTimeout(trackParentCtx, 5*time.Second)
			defer trackCancel()
			if err := s.habitTracker.TrackEventCreation(trackCtx, event); err != nil {
				s.logger.Error("failed to track event for habit detection (gRPC update)", "err", err)
			}
		}()
	}

	pbEvent := &pb.Event{
		Id:          event.ID,
		UserId:      event.UserID,
		Title:       event.Title,
		Description: event.Description,
		StartTime:   event.StartTime.Unix(),
		EndTime:     event.EndTime.Unix(),
		Location:    event.Location,
	}

	return &pb.UpdateEventResponse{
		Event:   pbEvent,
		Success: true,
		Message: "Event updated successfully",
	}, nil
}

// DeleteEvent implements CalendarService.DeleteEvent
func (s *Service) DeleteEvent(ctx context.Context, req *pb.DeleteEventRequest) (*pb.DeleteEventResponse, error) {
	s.logger.Info("DeleteEvent called via gRPC", "user_id", req.UserId, "event_id", req.Id)

	if err := s.eventRepo.DeleteEvent(ctx, req.Id); err != nil {
		s.logger.Error("failed to delete event via gRPC", "err", err)
		return &pb.DeleteEventResponse{
			Success: false,
			Message: fmt.Sprintf("failed to delete event: %v", err),
		}, nil
	}

	return &pb.DeleteEventResponse{
		Success: true,
		Message: "Event deleted successfully",
	}, nil
}

// GetAvailableSlots implements CalendarService.GetAvailableSlots
func (s *Service) GetAvailableSlots(ctx context.Context, req *pb.GetAvailableSlotsRequest) (*pb.GetAvailableSlotsResponse, error) {
	s.logger.Info("GetAvailableSlots called via gRPC", "user_id", req.UserId, "start_time", req.StartTime, "end_time", req.EndTime, "duration", req.Duration)

	if req.StartTime >= req.EndTime {
		return &pb.GetAvailableSlotsResponse{
			Slots:   []*pb.TimeSlot{},
			Success: false,
			Message: "start_time must be before end_time",
		}, nil
	}

	duration := req.Duration
	if duration <= 0 {
		duration = 3600 // default to 1 hour if not specified
	}

	rangeStart := time.Unix(req.StartTime, 0)
	rangeEnd := time.Unix(req.EndTime, 0)

	events, err := s.eventRepo.ListEvents(ctx, req.UserId, rangeStart, rangeEnd)
	if err != nil {
		s.logger.Error("failed to list events for GetAvailableSlots", "err", err)
		return &pb.GetAvailableSlotsResponse{
			Slots:   []*pb.TimeSlot{},
			Success: false,
			Message: fmt.Sprintf("failed to retrieve events: %v", err),
		}, nil
	}

	// Sort events by start time so we can sweep through them linearly
	sort.Slice(events, func(i, j int) bool {
		return events[i].StartTime.Before(events[j].StartTime)
	})

	var slots []*pb.TimeSlot
	cursor := req.StartTime // Unix timestamp of the current free-time start

	for _, event := range events {
		evStart := event.StartTime.Unix()
		evEnd := event.EndTime.Unix()

		// There is a free gap before this event
		if evStart > cursor && evStart-cursor >= duration {
			slots = append(slots, &pb.TimeSlot{
				StartTime: cursor,
				EndTime:   evStart,
			})
		}

		// Advance cursor past the end of this event (do not move it backwards)
		if evEnd > cursor {
			cursor = evEnd
		}
	}

	// Check the trailing free gap after the last event
	if req.EndTime > cursor && req.EndTime-cursor >= duration {
		slots = append(slots, &pb.TimeSlot{
			StartTime: cursor,
			EndTime:   req.EndTime,
		})
	}

	if slots == nil {
		slots = []*pb.TimeSlot{}
	}

	return &pb.GetAvailableSlotsResponse{
		Slots:   slots,
		Success: true,
		Message: fmt.Sprintf("Found %d available slot(s)", len(slots)),
	}, nil
}

// applyUpdateToEvent applies non-empty fields from req to the provided event and
// updates the UpdatedAt timestamp. Parsing errors for times are ignored (no-op).
func applyUpdateToEvent(event *models.Event, req *updateEventRequest) {
	if req.Title != "" {
		event.Title = req.Title
	}
	if req.Description != "" {
		event.Description = req.Description
	}
	if req.Location != "" {
		event.Location = req.Location
	}
	if req.Hashtags != nil {
		event.Hashtags = req.Hashtags
	}
	if req.IsRecurring {
		event.IsRecurring = req.IsRecurring
	}
	if req.RecurrenceRule != "" {
		event.RecurrenceRule = req.RecurrenceRule
	}
	if req.RecurrenceException != "" {
		event.RecurrenceException = req.RecurrenceException
	}
	if req.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, req.StartTime); err == nil {
			event.StartTime = t
		}
	}
	if req.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, req.EndTime); err == nil {
			event.EndTime = t
		}
	}
	event.UpdatedAt = time.Now()
}
