package habit

import (
	"context"
	"sync"
	"time"

	"github.com/waydxd/Orbit-core/internal/shared/models"
)

// mockRepo is a lightweight, test-only implementation of Repository.
// It exposes fields to control return values and record calls.
type mockRepo struct {
	mu sync.Mutex

	// Controlled responses
	getFreqByPatternResp       *models.EventFrequency
	getFreqByPatternErr        error
	upsertErr                  error
	updateFreqErr              error
	createHabitSuggestionErr   error
	getPendingSuggestionsResp  []*models.HabitSuggestion
	getPendingSuggestionsErr   error
	getHabitSuggestionByIDResp *models.HabitSuggestion
	getHabitSuggestionByIDErr  error
	updateHabitSuggestionErr   error
	createRecurringEventErr    error
	// If > 0, CreateRecurringEvent returns createRecurringEventErr starting at this call number (1-indexed)
	createRecurringEventFailAfterN int

	// Call records
	UpsertCalled                  bool
	LastUpsert                    *models.EventFrequency
	UpdateFreqCalled              bool
	LastUpdatedFreq               *models.EventFrequency
	CreateSuggestionCalled        bool
	LastCreatedSuggestion         *models.HabitSuggestion
	CreateRecurringEventCalled    bool
	CreateRecurringEventCallCount int
	LastCreatedEvent              *models.Event
	UpdateSuggestionStatusCalls   []updateSuggestionCall
	GetPendingCalled              bool
	GetHabitByIDCalled            bool
}

type updateSuggestionCall struct {
	id                string
	status            string
	recurrenceEndDate *time.Time
}

// Utilities to set expected return values
func (m *mockRepo) setGetFreqByPattern(resp *models.EventFrequency, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getFreqByPatternResp = resp
	m.getFreqByPatternErr = err
}

func (m *mockRepo) setGetHabitSuggestionByID(resp *models.HabitSuggestion, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getHabitSuggestionByIDResp = resp
	m.getHabitSuggestionByIDErr = err
}

// Repository interface methods
func (m *mockRepo) UpsertEventFrequency(ctx context.Context, freq *models.EventFrequency) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UpsertCalled = true
	m.LastUpsert = freq
	return m.upsertErr
}

func (m *mockRepo) GetEventFrequencyByPattern(ctx context.Context, userID, title string, durationMinutes, timeOfDay, dayOfWeek int) (*models.EventFrequency, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getFreqByPatternResp, m.getFreqByPatternErr
}

func (m *mockRepo) GetEventFrequenciesAboveThreshold(ctx context.Context, userID string, threshold int) ([]*models.EventFrequency, error) {
	return nil, nil
}

func (m *mockRepo) UpdateEventFrequency(ctx context.Context, freq *models.EventFrequency) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UpdateFreqCalled = true
	m.LastUpdatedFreq = freq
	return m.updateFreqErr
}

func (m *mockRepo) MarkEventsAsRecurringByPattern(_ context.Context, _ string, _ string, _ int, _ int, _ int) error {
	return nil
}

func (m *mockRepo) CreateHabitSuggestion(ctx context.Context, suggestion *models.HabitSuggestion) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CreateSuggestionCalled = true
	m.LastCreatedSuggestion = suggestion
	return m.createHabitSuggestionErr
}

func (m *mockRepo) GetPendingHabitSuggestions(ctx context.Context, userID string) ([]*models.HabitSuggestion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetPendingCalled = true
	return m.getPendingSuggestionsResp, m.getPendingSuggestionsErr
}

func (m *mockRepo) GetHabitSuggestionByID(ctx context.Context, id string) (*models.HabitSuggestion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetHabitByIDCalled = true
	if m.getHabitSuggestionByIDErr != nil {
		return nil, m.getHabitSuggestionByIDErr
	}
	if m.getHabitSuggestionByIDResp != nil {
		resp := *m.getHabitSuggestionByIDResp
		resp.ID = id
		return &resp, nil
	}
	return nil, nil
}

func (m *mockRepo) UpdateHabitSuggestionStatus(ctx context.Context, id, status string, recurrenceEndDate *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UpdateSuggestionStatusCalls = append(m.UpdateSuggestionStatusCalls, updateSuggestionCall{id: id, status: status, recurrenceEndDate: recurrenceEndDate})
	return m.updateHabitSuggestionErr
}

func (m *mockRepo) CreateRecurringEvent(ctx context.Context, event *models.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CreateRecurringEventCalled = true
	m.CreateRecurringEventCallCount++
	m.LastCreatedEvent = event
	if m.createRecurringEventFailAfterN == 0 || m.CreateRecurringEventCallCount >= m.createRecurringEventFailAfterN {
		return m.createRecurringEventErr
	}
	return nil
}

func (m *mockRepo) GetActiveRecurringEvents(ctx context.Context, userID string) ([]*models.Event, error) {
	return nil, nil
}

func (m *mockRepo) DeactivateRecurringEvent(ctx context.Context, eventID string) error {
	return nil
}

func (m *mockRepo) GetUserTimezone(ctx context.Context, userID string) (string, error) {
	return "UTC", nil
}

// Test helpers
func sampleEvent(userID, title string, start time.Time, durationMinutes int) *models.Event {
	return &models.Event{
		ID:          "evt-1",
		UserID:      userID,
		Title:       title,
		Description: "",
		StartTime:   start,
		EndTime:     start.Add(time.Duration(durationMinutes) * time.Minute),
		Location:    "",
	}
}

func sampleFreq(userID, title string, durationMinutes, timeOfDay, dayOfWeek, occurrenceCount int) *models.EventFrequency {
	now := time.Now()
	return &models.EventFrequency{
		ID:                   "freq-1",
		UserID:               userID,
		Title:                title,
		DurationMinutes:      durationMinutes,
		TimeOfDay:            timeOfDay,
		DayOfWeek:            dayOfWeek,
		OccurrenceCount:      occurrenceCount,
		SuggestionThreshold:  DefaultSuggestionThreshold,
		SuggestionShown:      false,
		HabitAccepted:        false,
		OccurrenceTimestamps: []time.Time{now},
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

func sampleSuggestion(userID, freqID, title string, durationMinutes, timeOfDay, dayOfWeek int, status string) *models.HabitSuggestion {
	now := time.Now()
	return &models.HabitSuggestion{
		ID:               "sugg-1",
		UserID:           userID,
		EventFrequencyID: freqID,
		Title:            title,
		DurationMinutes:  durationMinutes,
		TimeOfDay:        timeOfDay,
		DayOfWeek:        dayOfWeek,
		Status:           status,
		CreatedAt:        now,
		UpdatedAt:        now,
		ExpiresAt:        now.Add(7 * 24 * time.Hour),
	}
}
