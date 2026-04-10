package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
)

func TestNewService(t *testing.T) {
	cfg := &config.Config{}
	log := logger.New()

	svc := NewService(cfg, log)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}

	if svc.config != cfg {
		t.Error("expected config to be set")
	}

	if svc.logger != log {
		t.Error("expected logger to be set")
	}
}

type mockCalSvc struct{}

func (m *mockCalSvc) ListEventsAdapter(ctx context.Context, startTime, endTime int64, status string) ([]interface{}, error) {
	return nil, nil
}

func (m *mockCalSvc) CreateEventAdapter(ctx context.Context, event interface{}) (interface{}, error) {
	return nil, nil
}

func (m *mockCalSvc) UpdateEventAdapter(ctx context.Context, id string, event interface{}) (interface{}, error) {
	return nil, nil
}

func (m *mockCalSvc) DeleteEventAdapter(ctx context.Context, id string) error {
	return nil
}

func TestSetCalendarService(t *testing.T) {
	cfg := &config.Config{}
	log := logger.New()
	svc := NewService(cfg, log)

	mock := &mockCalSvc{}
	svc.SetCalendarService(mock)

	if svc.calendarService != mock {
		t.Error("expected calendar service to be set")
	}
}

func TestRespondWithJSON(t *testing.T) {
	cfg := &config.Config{}
	log := logger.New()
	svc := NewService(cfg, log)

	w := httptest.NewRecorder()
	svc.respondWithJSON(w, http.StatusOK, map[string]string{"message": "test"})

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp["message"] != "test" {
		t.Errorf("message = %q, want test", resp["message"])
	}
}

func TestRespondWithError(t *testing.T) {
	cfg := &config.Config{}
	log := logger.New()
	svc := NewService(cfg, log)

	w := httptest.NewRecorder()
	svc.respondWithError(w, http.StatusBadRequest, "test error", nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp["error"] != "test error" {
		t.Errorf("error = %q, want test error", resp["error"])
	}
}

func TestParseExportParams(t *testing.T) {
	cfg := &config.Config{}
	log := logger.New()
	svc := NewService(cfg, log)

	tests := []struct {
		name           string
		query          string
		wantFormat     string
		wantStatusCode int
	}{
		{"default ics", "", "ics", 0},
		{"explicit csv", "?format=csv", "csv", 0},
		{"invalid format", "?format=xml", "", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/export"+tt.query, nil)
			format, start, end, status, err := svc.parseExportParams(r)

			if tt.wantStatusCode != 0 {
				if status != tt.wantStatusCode {
					t.Errorf("status = %d, want %d", status, tt.wantStatusCode)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if format != tt.wantFormat {
				t.Errorf("format = %q, want %q", format, tt.wantFormat)
			}

			if start == 0 || end == 0 {
				t.Error("expected non-zero times")
			}
		})
	}
}

func TestGenerateExportData(t *testing.T) {
	cfg := &config.Config{}
	log := logger.New()
	svc := NewService(cfg, log)

	_, format, _, err := svc.generateExportData("ics", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != "text/calendar; charset=utf-8" {
		t.Errorf("format = %q, want text/calendar; charset=utf-8", format)
	}

	_, format, _, err = svc.generateExportData("csv", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != "text/csv; charset=utf-8" {
		t.Errorf("format = %q, want text/csv; charset=utf-8", format)
	}

	_, _, _, err = svc.generateExportData("invalid", nil)
	if err == nil {
		t.Error("expected error for invalid format")
	}
}

func TestParseSyncTimeRange(t *testing.T) {
	cfg := &config.Config{}
	log := logger.New()
	svc := NewService(cfg, log)

	start, end := svc.parseSyncTimeRange("", "")

	if start.IsZero() || end.IsZero() {
		t.Error("expected non-zero times with empty inputs")
	}

	start, end = svc.parseSyncTimeRange("2024-01-01T00:00:00Z", "2024-12-31T23:59:59Z")

	if start.Year() != 2024 || start.Month() != 1 {
		t.Errorf("start = %v, want 2024-01", start)
	}

	if end.Year() != 2024 || end.Month() != 12 {
		t.Errorf("end = %v, want 2024-12", end)
	}
}

func TestExecuteGoogleSync_InvalidDirection(t *testing.T) {
	cfg := &config.Config{}
	log := logger.New()
	svc := NewService(cfg, log)

	result, err := svc.executeGoogleSync(nil, "user123", "invalid", time.Time{}, time.Time{})
	if err == nil {
		t.Error("expected error for invalid direction")
	}
	if result != nil {
		t.Error("expected nil result for invalid direction")
	}
}

func TestPerformBidirectionalSync(t *testing.T) {
	cfg := &config.Config{}
	log := logger.New()
	svc := NewService(cfg, log)

	result := svc.performBidirectionalSync(nil, "user123", time.Time{}, time.Time{})

	if result["direction"] != "bidirectional" {
		t.Errorf("direction = %q, want bidirectional", result["direction"])
	}
}

func TestSyncData_InvalidJSON(t *testing.T) {
	cfg := &config.Config{}
	log := logger.New()
	svc := NewService(cfg, log)

	r := httptest.NewRequest("POST", "/sync", strings.NewReader(`invalid json`))
	r.Body = r.Body

	w := httptest.NewRecorder()
	svc.syncData(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleWebhook(t *testing.T) {
	cfg := &config.Config{}
	log := logger.New()
	svc := NewService(cfg, log)

	r := httptest.NewRequest("POST", "/webhook", strings.NewReader(`invalid json`))

	w := httptest.NewRecorder()
	svc.handleWebhook(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGetExternalStatus(t *testing.T) {
	cfg := &config.Config{}
	log := logger.New()
	svc := NewService(cfg, log)

	r := httptest.NewRequest("GET", "/external/status", nil)
	w := httptest.NewRecorder()

	svc.getExternalStatus(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRegisterRoutes(t *testing.T) {
	cfg := &config.Config{}
	log := logger.New()
	svc := NewService(cfg, log)

	router := mux.NewRouter()
	svc.RegisterRoutes(router)

	if router == nil {
		t.Fatal("expected non-nil router")
	}
}
