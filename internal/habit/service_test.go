package habit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/logger"
	"github.com/waydxd/Orbit-core/pkg/middleware"
)

func TestTrackEventCreation_NewFrequency(t *testing.T) {
	mock := &mockRepo{}
	svc := NewService(nil, logger.New(), mock)

	now := time.Now()
	evt := sampleEvent("user-1", "Yoga", now, 60)

	// Repo returns nil existing frequency
	mock.setGetFreqByPattern(nil, nil)

	if err := svc.TrackEventCreation(context.Background(), evt); err != nil {
		t.Fatalf("TrackEventCreation returned error: %v", err)
	}

	if !mock.UpsertCalled {
		t.Fatalf("expected UpsertEventFrequency to be called")
	}

	if mock.LastUpsert == nil {
		t.Fatalf("expected last upsert to be set")
	}

	if mock.LastUpsert.OccurrenceCount != 1 {
		t.Fatalf("expected occurrence count 1, got %d", mock.LastUpsert.OccurrenceCount)
	}
}

func TestTrackEventCreation_ExistingFrequency_TriggersSuggestion(t *testing.T) {
	mock := &mockRepo{}
	svc := NewService(nil, logger.New(), mock)

	// existing with OccurrenceCount 2 and threshold 3 -> after increment becomes 3 and should trigger
	existing := sampleFreq("user-2", "Run", 60, 9*60, 1, 2)
	mock.setGetFreqByPattern(existing, nil)

	start := time.Now()
	evt := sampleEvent("user-2", "Run", start, 60)

	if err := svc.TrackEventCreation(context.Background(), evt); err != nil {
		t.Fatalf("TrackEventCreation returned error: %v", err)
	}

	if !mock.UpdateFreqCalled {
		t.Fatalf("expected UpdateEventFrequency to be called")
	}

	if !mock.CreateSuggestionCalled {
		t.Fatalf("expected CreateHabitSuggestion to be called when threshold reached")
	}

	if mock.LastCreatedSuggestion == nil {
		t.Fatalf("expected a created suggestion")
	}

	if mock.LastCreatedSuggestion.Title != "Run" {
		t.Fatalf("expected suggestion title Run, got %s", mock.LastCreatedSuggestion.Title)
	}
}

func TestAcceptSuggestion_HappyPath(t *testing.T) {
	mock := &mockRepo{}
	svc := NewService(nil, logger.New(), mock)

	sug := sampleSuggestion("user-3", "freq-3", "Meditate", 30, 8*60, 1, "pending")
	mock.setGetHabitSuggestionByID(sug, nil)

	// Ensure CreateRecurringEvent succeeds
	mock.createRecurringEventErr = nil

	// Also return a freq so UpdateEventFrequency will be invoked
	mock.setGetFreqByPattern(sampleFreq("user-3", "Meditate", 30, 8*60, 1, 3), nil)

	event, err := svc.AcceptSuggestion(context.Background(), "sugg-1")
	if err != nil {
		t.Fatalf("AcceptSuggestion returned error: %v", err)
	}

	if event == nil {
		t.Fatalf("expected event to be returned")
	}

	if !mock.CreateRecurringEventCalled {
		t.Fatalf("expected CreateRecurringEvent to be called")
	}

	if len(mock.UpdateSuggestionStatusCalls) == 0 || mock.UpdateSuggestionStatusCalls[0].status != "accepted" {
		t.Fatalf("expected UpdateHabitSuggestionStatus called with accepted")
	}

	// RRULE checks
	if !strings.Contains(event.RecurrenceRule, "FREQ=WEEKLY") {
		t.Fatalf("expected RRULE to contain FREQ=WEEKLY, got %s", event.RecurrenceRule)
	}

	if !strings.Contains(event.RecurrenceRule, "BYDAY=") {
		t.Fatalf("expected RRULE to contain BYDAY, got %s", event.RecurrenceRule)
	}

	// UNTIL should be an 8-digit date
	if idx := strings.Index(event.RecurrenceRule, "UNTIL="); idx >= 0 {
		until := event.RecurrenceRule[idx+6:]
		if len(until) < 8 {
			t.Fatalf("expected UNTIL to contain YYYYMMDD, got %s", until)
		}
	} else {
		t.Fatalf("expected RRULE to contain UNTIL")
	}
}

func TestAcceptSuggestion_NotFound(t *testing.T) {
	mock := &mockRepo{}
	svc := NewService(nil, logger.New(), mock)

	mock.setGetHabitSuggestionByID(nil, errNotFound())

	_, err := svc.AcceptSuggestion(context.Background(), "missing")
	if err == nil {
		t.Fatalf("expected error when suggestion not found")
	}
}

func TestRejectSuggestion_HappyPath(t *testing.T) {
	mock := &mockRepo{}
	svc := NewService(nil, logger.New(), mock)

	sug := sampleSuggestion("user-4", "freq-4", "Stretch", 20, 7*60, 0, "pending")
	mock.setGetHabitSuggestionByID(sug, nil)

	if err := svc.RejectSuggestion(context.Background(), "sugg-1"); err != nil {
		t.Fatalf("RejectSuggestion returned error: %v", err)
	}

	if len(mock.UpdateSuggestionStatusCalls) == 0 || mock.UpdateSuggestionStatusCalls[0].status != "rejected" {
		t.Fatalf("expected UpdateHabitSuggestionStatus called with rejected")
	}
}

func TestHandleGetSuggestions_Authorization(t *testing.T) {
	mock := &mockRepo{}
	svc := NewService(nil, logger.New(), mock)

	req := httptest.NewRequest("GET", "/habit/suggestions", nil)
	rec := httptest.NewRecorder()

	svc.handleGetSuggestions(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when unauthenticated, got %d", rec.Code)
	}

	// Now authenticate and return one suggestion
	userID := "user-5"
	sug := sampleSuggestion(userID, "f", "Walk", 30, 18*60, 2, "pending")
	mock.getPendingSuggestionsResp = []*models.HabitSuggestion{sug}

	req = httptest.NewRequest("GET", "/habit/suggestions", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()

	svc.handleGetSuggestions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when authenticated, got %d", rec.Code)
	}

	var body []map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(body) != 1 {
		t.Fatalf("expected one suggestion in response, got %d", len(body))
	}
}

func TestHandleAcceptSuggestion_AuthAndOwnership(t *testing.T) {
	mock := &mockRepo{}
	svc := NewService(nil, logger.New(), mock)

	// Unauthenticated
	req := httptest.NewRequest("POST", "/habit/suggestions/s1/accept", nil)
	rec := httptest.NewRecorder()
	svc.handleAcceptSuggestion(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when unauthenticated, got %d", rec.Code)
	}

	// Forbidden when suggestion belongs to someone else
	userID := "owner-1"
	sug := sampleSuggestion("other-user", "freq-x", "DoThings", 10, 9*60, 1, "pending")
	mock.setGetHabitSuggestionByID(sug, nil)

	req = httptest.NewRequest("POST", "/habit/suggestions/s1/accept", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	svc.handleAcceptSuggestion(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when suggestion owned by another user, got %d", rec.Code)
	}
}

// errNotFound returns an error matching repository not found semantics
func errNotFound() error {
	return &notFoundError{}
}

type notFoundError struct{}

func (n *notFoundError) Error() string { return "not found" }

// Ensure notFoundError isn't used elsewhere; it's only for testing.
