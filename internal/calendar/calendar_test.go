package calendar

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
	pb "github.com/waydxd/Orbit-core/proto/calendar"
)

// ===== Mock implementations =====

type mockEventRepo struct {
	events       []*models.Event
	err          error
	createErr    error
	updateErr    error
	getByIDErr   error
	getByIDResp  *models.Event
	createdEvent *models.Event
	updatedEvent *models.Event
}

func (m *mockEventRepo) CreateEvent(_ context.Context, event *models.Event) error {
	m.createdEvent = event
	return m.createErr
}
func (m *mockEventRepo) GetEventByID(_ context.Context, _ string) (*models.Event, error) {
	return m.getByIDResp, m.getByIDErr
}
func (m *mockEventRepo) ListEvents(_ context.Context, _ string, _, _ time.Time) ([]*models.Event, error) {
	return m.events, m.err
}
func (m *mockEventRepo) UpdateEvent(_ context.Context, event *models.Event) error {
	m.updatedEvent = event
	return m.updateErr
}
func (m *mockEventRepo) DeleteEvent(_ context.Context, _ string) error        { return nil }
func (m *mockEventRepo) GetActiveRecurringEvents(_ context.Context, _ string) ([]*models.Event, error) {
	return nil, nil
}
func (m *mockEventRepo) DeactivateRecurringEvent(_ context.Context, _ string) error { return nil }

type mockTaskRepo struct{}

func (m *mockTaskRepo) CreateTask(_ context.Context, _ *models.Task) error { return nil }
func (m *mockTaskRepo) GetTaskByID(_ context.Context, _ string) (*models.Task, error) {
	return nil, nil
}
func (m *mockTaskRepo) ListTasks(_ context.Context, _ string, _ *bool) ([]*models.Task, error) {
	return nil, nil
}
func (m *mockTaskRepo) UpdateTask(_ context.Context, _ *models.Task) error { return nil }
func (m *mockTaskRepo) DeleteTask(_ context.Context, _ string) error       { return nil }

type mockHabitTracker struct {
	calls int
}

func (m *mockHabitTracker) TrackEventCreation(_ context.Context, _ *models.Event) error {
	m.calls++
	return nil
}

func newTestCalendarService(events []*models.Event, listErr error) *Service {
	return NewService(
		&config.Config{},
		logger.New(),
		&mockEventRepo{events: events, err: listErr},
		&mockTaskRepo{},
		&mockHabitTracker{},
	)
}

func TestCreateEventAdapter_DuplicateIDUpdatesExistingEvent(t *testing.T) {
	repo := &mockEventRepo{
		createErr:   &pgconn.PgError{Code: "23505"},
		getByIDResp: &models.Event{ID: "event-1", UserID: "user-1"},
	}
	habitTracker := &mockHabitTracker{}
	svc := NewService(&config.Config{}, logger.New(), repo, &mockTaskRepo{}, habitTracker)

	event := &models.Event{
		ID:        "event-1",
		UserID:    "user-1",
		Title:     "Updated title",
		StartTime: time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2025, 1, 10, 11, 0, 0, 0, time.UTC),
	}

	result, err := svc.CreateEventAdapter(context.Background(), event)
	if err != nil {
		t.Fatalf("CreateEventAdapter returned error: %v", err)
	}
	created, ok := result.(*models.Event)
	if !ok {
		t.Fatalf("expected *models.Event result, got %T", result)
	}
	if repo.updatedEvent == nil {
		t.Fatal("expected duplicate event to be updated")
	}
	if repo.updatedEvent.Title != "Updated title" {
		t.Errorf("updated title = %q, want %q", repo.updatedEvent.Title, "Updated title")
	}
	if habitTracker.calls != 0 {
		t.Fatalf("expected habit tracker to be skipped for duplicate import, got %d calls", habitTracker.calls)
	}
	if created.ID != event.ID {
		t.Errorf("result ID = %q, want %q", created.ID, event.ID)
	}
}

// ===== GetAvailableSlots tests =====

func TestGetAvailableSlots_NoEvents_ReturnsSingleSlot(t *testing.T) {
	start := time.Date(2025, 1, 10, 9, 0, 0, 0, time.UTC).Unix()
	end := time.Date(2025, 1, 10, 17, 0, 0, 0, time.UTC).Unix()
	duration := int64(3600) // 1 hour

	svc := newTestCalendarService(nil, nil)
	resp, err := svc.GetAvailableSlots(context.Background(), &pb.GetAvailableSlotsRequest{
		UserId:    "user-1",
		StartTime: start,
		EndTime:   end,
		Duration:  duration,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got message: %s", resp.Message)
	}
	if len(resp.Slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(resp.Slots))
	}
	if resp.Slots[0].StartTime != start {
		t.Errorf("slot start: want %d, got %d", start, resp.Slots[0].StartTime)
	}
	if resp.Slots[0].EndTime != end {
		t.Errorf("slot end: want %d, got %d", end, resp.Slots[0].EndTime)
	}
}

func TestGetAvailableSlots_EventsFillingRange_ReturnsNoSlots(t *testing.T) {
	start := time.Date(2025, 1, 10, 9, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 10, 17, 0, 0, 0, time.UTC)

	events := []*models.Event{
		{StartTime: start, EndTime: end},
	}

	svc := newTestCalendarService(events, nil)
	resp, err := svc.GetAvailableSlots(context.Background(), &pb.GetAvailableSlotsRequest{
		UserId:    "user-1",
		StartTime: start.Unix(),
		EndTime:   end.Unix(),
		Duration:  3600,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success flag")
	}
	if len(resp.Slots) != 0 {
		t.Fatalf("expected 0 slots when events fill the range, got %d", len(resp.Slots))
	}
}

func TestGetAvailableSlots_EventInMiddle_ReturnsTwoSlots(t *testing.T) {
	rangeStart := time.Date(2025, 1, 10, 9, 0, 0, 0, time.UTC)
	eventStart := time.Date(2025, 1, 10, 11, 0, 0, 0, time.UTC)
	eventEnd := time.Date(2025, 1, 10, 13, 0, 0, 0, time.UTC)
	rangeEnd := time.Date(2025, 1, 10, 17, 0, 0, 0, time.UTC)

	events := []*models.Event{
		{StartTime: eventStart, EndTime: eventEnd},
	}

	svc := newTestCalendarService(events, nil)
	resp, err := svc.GetAvailableSlots(context.Background(), &pb.GetAvailableSlotsRequest{
		UserId:    "user-1",
		StartTime: rangeStart.Unix(),
		EndTime:   rangeEnd.Unix(),
		Duration:  3600, // 1 hour
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success")
	}
	if len(resp.Slots) != 2 {
		t.Fatalf("expected 2 slots, got %d", len(resp.Slots))
	}
	// First slot: rangeStart -> eventStart
	if resp.Slots[0].StartTime != rangeStart.Unix() || resp.Slots[0].EndTime != eventStart.Unix() {
		t.Errorf("first slot mismatch: got [%d, %d]", resp.Slots[0].StartTime, resp.Slots[0].EndTime)
	}
	// Second slot: eventEnd -> rangeEnd
	if resp.Slots[1].StartTime != eventEnd.Unix() || resp.Slots[1].EndTime != rangeEnd.Unix() {
		t.Errorf("second slot mismatch: got [%d, %d]", resp.Slots[1].StartTime, resp.Slots[1].EndTime)
	}
}

func TestGetAvailableSlots_GapShorterThanDuration_GapExcluded(t *testing.T) {
	rangeStart := time.Date(2025, 1, 10, 9, 0, 0, 0, time.UTC)
	// Two back-to-back events leaving a 30-minute gap in the middle
	event1Start := time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC)
	event1End := time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)
	event2Start := time.Date(2025, 1, 10, 12, 30, 0, 0, time.UTC)
	event2End := time.Date(2025, 1, 10, 14, 0, 0, 0, time.UTC)
	rangeEnd := time.Date(2025, 1, 10, 17, 0, 0, 0, time.UTC)

	events := []*models.Event{
		{StartTime: event1Start, EndTime: event1End},
		{StartTime: event2Start, EndTime: event2End},
	}

	svc := newTestCalendarService(events, nil)
	resp, err := svc.GetAvailableSlots(context.Background(), &pb.GetAvailableSlotsRequest{
		UserId:    "user-1",
		StartTime: rangeStart.Unix(),
		EndTime:   rangeEnd.Unix(),
		Duration:  3600, // 1 hour — the 30-min gap should NOT appear
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success")
	}
	// Only the leading slot (9-10) and trailing slot (14-17) qualify
	if len(resp.Slots) != 2 {
		t.Fatalf("expected 2 slots (leading + trailing), got %d", len(resp.Slots))
	}
	if resp.Slots[0].StartTime != rangeStart.Unix() || resp.Slots[0].EndTime != event1Start.Unix() {
		t.Errorf("leading slot mismatch: got [%d, %d]", resp.Slots[0].StartTime, resp.Slots[0].EndTime)
	}
	if resp.Slots[1].StartTime != event2End.Unix() || resp.Slots[1].EndTime != rangeEnd.Unix() {
		t.Errorf("trailing slot mismatch: got [%d, %d]", resp.Slots[1].StartTime, resp.Slots[1].EndTime)
	}
}

func TestGetAvailableSlots_InvalidRange_ReturnsError(t *testing.T) {
	svc := newTestCalendarService(nil, nil)
	resp, err := svc.GetAvailableSlots(context.Background(), &pb.GetAvailableSlotsRequest{
		UserId:    "user-1",
		StartTime: 1000,
		EndTime:   500, // end before start
		Duration:  3600,
	})

	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if resp.Success {
		t.Fatal("expected failure for invalid range")
	}
}

func TestGetAvailableSlots_OverlappingEvents_HandledCorrectly(t *testing.T) {
	rangeStart := time.Date(2025, 1, 10, 9, 0, 0, 0, time.UTC)
	rangeEnd := time.Date(2025, 1, 10, 17, 0, 0, 0, time.UTC)

	// Two overlapping events — should be treated as one blocked block
	events := []*models.Event{
		{
			StartTime: time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC),
			EndTime:   time.Date(2025, 1, 10, 13, 0, 0, 0, time.UTC),
		},
		{
			StartTime: time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC),
			EndTime:   time.Date(2025, 1, 10, 14, 0, 0, 0, time.UTC),
		},
	}

	svc := newTestCalendarService(events, nil)
	resp, err := svc.GetAvailableSlots(context.Background(), &pb.GetAvailableSlotsRequest{
		UserId:    "user-1",
		StartTime: rangeStart.Unix(),
		EndTime:   rangeEnd.Unix(),
		Duration:  3600,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success")
	}
	// Leading slot 9-10 and trailing slot 14-17
	if len(resp.Slots) != 2 {
		t.Fatalf("expected 2 slots for overlapping events, got %d", len(resp.Slots))
	}
	if resp.Slots[1].StartTime != time.Date(2025, 1, 10, 14, 0, 0, 0, time.UTC).Unix() {
		t.Errorf("trailing slot should start at 14:00, got %d", resp.Slots[1].StartTime)
	}
}

func TestGetAvailableSlots_DefaultDurationApplied(t *testing.T) {
	// duration = 0 should default to 3600s
	rangeStart := time.Date(2025, 1, 10, 9, 0, 0, 0, time.UTC)
	rangeEnd := time.Date(2025, 1, 10, 10, 30, 0, 0, time.UTC) // 1.5 hours total

	svc := newTestCalendarService(nil, nil)
	resp, err := svc.GetAvailableSlots(context.Background(), &pb.GetAvailableSlotsRequest{
		UserId:    "user-1",
		StartTime: rangeStart.Unix(),
		EndTime:   rangeEnd.Unix(),
		Duration:  0, // should default to 3600s
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success")
	}
	// 1.5-hour range >= 1-hour default duration → one slot
	if len(resp.Slots) != 1 {
		t.Fatalf("expected 1 slot when range > default duration, got %d", len(resp.Slots))
	}
}
