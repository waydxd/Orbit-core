package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/waydxd/Orbit-core/pkg/logger"
)

// ===== mock services =====

type mockAuthService struct{}

func (m *mockAuthService) RegisterRoutes(_ *mux.Router)          {}
func (m *mockAuthService) RegisterProtectedRoutes(_ *mux.Router) {}

type mockCalendarService struct{}

func (m *mockCalendarService) RegisterRoutes(_ *mux.Router) {}
func (m *mockCalendarService) ListEventsAdapter(_ context.Context, _, _ int64, _ string) ([]interface{}, error) {
	return nil, nil
}
func (m *mockCalendarService) CreateEventAdapter(_ context.Context, _ interface{}) (interface{}, error) {
	return nil, nil
}
func (m *mockCalendarService) UpdateEventAdapter(_ context.Context, _ string, _ interface{}) (interface{}, error) {
	return nil, nil
}
func (m *mockCalendarService) DeleteEventAdapter(_ context.Context, _ string) error { return nil }

type mockLocationService struct{}

func (m *mockLocationService) RegisterRoutes(_ *mux.Router) {}

type mockIntegrationService struct{}

func (m *mockIntegrationService) RegisterRoutes(_ *mux.Router) {}

type mockChatService struct{}

func (m *mockChatService) RegisterRoutes(_ *mux.Router) {}

type mockHabitService struct{}

func (m *mockHabitService) RegisterRoutes(_ *mux.Router) {}

type mockNotificationService struct{}

func (m *mockNotificationService) RegisterRoutes(_ *mux.Router) {}

type mockAssetService struct{}

func (m *mockAssetService) RegisterRoutes(_ *mux.Router) {}

// ===== healthCheck tests =====

func TestHealthCheck_ReturnsHealthy(t *testing.T) {
	svc := &Service{logger: logger.New()}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	svc.healthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Fatalf("expected status=healthy, got %q", resp["status"])
	}
	if resp["service"] != "gateway" {
		t.Fatalf("expected service=gateway, got %q", resp["service"])
	}
}

func TestHealthCheck_ContentType(t *testing.T) {
	svc := &Service{logger: logger.New()}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	svc.healthCheck(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected Content-Type=application/json, got %q", ct)
	}
}

// ===== Router tests =====

func TestRouter_HealthCheckRoute(t *testing.T) {
	svc := &Service{
		logger: logger.New(),
		router: mux.NewRouter(),
		services: ServiceConfig{
			AuthService:         &mockAuthService{},
			CalendarService:     &mockCalendarService{},
			LocationService:     &mockLocationService{},
			IntegrationService:  &mockIntegrationService{},
			ChatService:         &mockChatService{},
			HabitService:        &mockHabitService{},
			NotificationService: &mockNotificationService{},
			AssetService:        &mockAssetService{},
		},
	}
	svc.setupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from /health via router, got %d", rr.Code)
	}
}

func TestRouter_Returns404ForUnknownRoute(t *testing.T) {
	svc := &Service{
		logger: logger.New(),
		router: mux.NewRouter(),
		services: ServiceConfig{
			AuthService:         &mockAuthService{},
			CalendarService:     &mockCalendarService{},
			LocationService:     &mockLocationService{},
			IntegrationService:  &mockIntegrationService{},
			ChatService:         &mockChatService{},
			HabitService:        &mockHabitService{},
			NotificationService: &mockNotificationService{},
			AssetService:        &mockAssetService{},
		},
	}
	svc.setupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/nonexistent-path-xyz", nil)
	rr := httptest.NewRecorder()
	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown route, got %d", rr.Code)
	}
}
