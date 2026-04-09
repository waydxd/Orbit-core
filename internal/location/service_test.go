package location

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/logger"
	"github.com/waydxd/Orbit-core/pkg/middleware"
)

// ===== mock repository =====

type mockLocationRepo struct {
	createErr       error
	historyResp     []*models.Location
	historyErr      error
	currentResp     *models.Location
	currentErr      error
	nearbyResp      []*models.Location
	nearbyErr       error
}

func (m *mockLocationRepo) CreateLocation(_ context.Context, _ *models.Location) error {
	return m.createErr
}

func (m *mockLocationRepo) GetLocationByID(_ context.Context, _ string) (*models.Location, error) {
	return nil, nil
}

func (m *mockLocationRepo) GetLocationHistory(_ context.Context, _ string, _ int) ([]*models.Location, error) {
	return m.historyResp, m.historyErr
}

func (m *mockLocationRepo) GetCurrentLocation(_ context.Context, _ string) (*models.Location, error) {
	return m.currentResp, m.currentErr
}

func (m *mockLocationRepo) FindNearby(_ context.Context, _, _, _ float64) ([]*models.Location, error) {
	return m.nearbyResp, m.nearbyErr
}

// ===== helpers =====

func newTestLocationService(repo Repository) *Service {
	return NewService(nil, logger.New(), repo)
}

func withUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
	return r.WithContext(ctx)
}

// ===== trackLocation =====

func TestTrackLocation_Unauthorized(t *testing.T) {
	svc := newTestLocationService(&mockLocationRepo{})
	req := httptest.NewRequest(http.MethodPost, "/location/track", nil)
	rr := httptest.NewRecorder()
	svc.trackLocation(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestTrackLocation_InvalidBody(t *testing.T) {
	svc := newTestLocationService(&mockLocationRepo{})
	req := httptest.NewRequest(http.MethodPost, "/location/track", bytes.NewBufferString("not-json"))
	req = withUserID(req, "user-1")
	rr := httptest.NewRecorder()
	svc.trackLocation(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestTrackLocation_RepoError(t *testing.T) {
	mock := &mockLocationRepo{createErr: errors.New("db error")}
	svc := newTestLocationService(mock)

	body, _ := json.Marshal(LocationRequest{Latitude: 1.0, Longitude: 2.0})
	req := httptest.NewRequest(http.MethodPost, "/location/track", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	rr := httptest.NewRecorder()
	svc.trackLocation(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestTrackLocation_Success(t *testing.T) {
	svc := newTestLocationService(&mockLocationRepo{})

	body, _ := json.Marshal(LocationRequest{Latitude: 22.3, Longitude: 114.2, Address: "HK"})
	req := httptest.NewRequest(http.MethodPost, "/location/track", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	rr := httptest.NewRecorder()
	svc.trackLocation(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var loc models.Location
	if err := json.NewDecoder(rr.Body).Decode(&loc); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}
	if loc.UserID != "user-1" {
		t.Fatalf("expected user_id=user-1, got %q", loc.UserID)
	}
}

// ===== getLocationHistory =====

func TestGetLocationHistory_Unauthorized(t *testing.T) {
	svc := newTestLocationService(&mockLocationRepo{})
	req := httptest.NewRequest(http.MethodGet, "/location/history", nil)
	rr := httptest.NewRecorder()
	svc.getLocationHistory(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestGetLocationHistory_RepoError(t *testing.T) {
	mock := &mockLocationRepo{historyErr: errors.New("db error")}
	svc := newTestLocationService(mock)

	req := httptest.NewRequest(http.MethodGet, "/location/history", nil)
	req = withUserID(req, "user-1")
	rr := httptest.NewRecorder()
	svc.getLocationHistory(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestGetLocationHistory_Success(t *testing.T) {
	locs := []*models.Location{{ID: "loc-1", UserID: "user-1"}}
	mock := &mockLocationRepo{historyResp: locs}
	svc := newTestLocationService(mock)

	req := httptest.NewRequest(http.MethodGet, "/location/history", nil)
	req = withUserID(req, "user-1")
	rr := httptest.NewRecorder()
	svc.getLocationHistory(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var result []*models.Location
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if len(result) != 1 || result[0].ID != "loc-1" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestGetLocationHistory_NilReturnsEmptyArray(t *testing.T) {
	mock := &mockLocationRepo{historyResp: nil}
	svc := newTestLocationService(mock)

	req := httptest.NewRequest(http.MethodGet, "/location/history", nil)
	req = withUserID(req, "user-1")
	rr := httptest.NewRecorder()
	svc.getLocationHistory(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var result []*models.Location
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if result == nil || len(result) != 0 {
		t.Fatalf("expected empty array, got %v", result)
	}
}

// ===== getCurrentLocation =====

func TestGetCurrentLocation_Unauthorized(t *testing.T) {
	svc := newTestLocationService(&mockLocationRepo{})
	req := httptest.NewRequest(http.MethodGet, "/location/current", nil)
	rr := httptest.NewRecorder()
	svc.getCurrentLocation(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestGetCurrentLocation_NotFound(t *testing.T) {
	mock := &mockLocationRepo{currentErr: errors.New("not found")}
	svc := newTestLocationService(mock)

	req := httptest.NewRequest(http.MethodGet, "/location/current", nil)
	req = withUserID(req, "user-1")
	rr := httptest.NewRecorder()
	svc.getCurrentLocation(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestGetCurrentLocation_Success(t *testing.T) {
	mock := &mockLocationRepo{currentResp: &models.Location{ID: "loc-curr", UserID: "user-1"}}
	svc := newTestLocationService(mock)

	req := httptest.NewRequest(http.MethodGet, "/location/current", nil)
	req = withUserID(req, "user-1")
	rr := httptest.NewRecorder()
	svc.getCurrentLocation(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var loc models.Location
	if err := json.NewDecoder(rr.Body).Decode(&loc); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if loc.ID != "loc-curr" {
		t.Fatalf("expected loc-curr, got %q", loc.ID)
	}
}

// ===== findNearby =====

func TestFindNearby_MissingParams(t *testing.T) {
	svc := newTestLocationService(&mockLocationRepo{})
	req := httptest.NewRequest(http.MethodGet, "/location/nearby", nil)
	rr := httptest.NewRecorder()
	svc.findNearby(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestFindNearby_InvalidLatitude(t *testing.T) {
	svc := newTestLocationService(&mockLocationRepo{})
	req := httptest.NewRequest(http.MethodGet, "/location/nearby?latitude=abc&longitude=1.0", nil)
	rr := httptest.NewRecorder()
	svc.findNearby(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestFindNearby_InvalidLongitude(t *testing.T) {
	svc := newTestLocationService(&mockLocationRepo{})
	req := httptest.NewRequest(http.MethodGet, "/location/nearby?latitude=1.0&longitude=xyz", nil)
	rr := httptest.NewRecorder()
	svc.findNearby(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestFindNearby_RepoError(t *testing.T) {
	mock := &mockLocationRepo{nearbyErr: errors.New("db error")}
	svc := newTestLocationService(mock)

	req := httptest.NewRequest(http.MethodGet, "/location/nearby?latitude=22.3&longitude=114.2", nil)
	rr := httptest.NewRecorder()
	svc.findNearby(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestFindNearby_Success(t *testing.T) {
	locs := []*models.Location{{ID: "near-1"}}
	mock := &mockLocationRepo{nearbyResp: locs}
	svc := newTestLocationService(mock)

	req := httptest.NewRequest(http.MethodGet, "/location/nearby?latitude=22.3&longitude=114.2&radius=5.0", nil)
	rr := httptest.NewRecorder()
	svc.findNearby(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var result []*models.Location
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if len(result) != 1 || result[0].ID != "near-1" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestFindNearby_NilReturnsEmptyArray(t *testing.T) {
	mock := &mockLocationRepo{nearbyResp: nil}
	svc := newTestLocationService(mock)

	req := httptest.NewRequest(http.MethodGet, "/location/nearby?latitude=1.0&longitude=1.0", nil)
	rr := httptest.NewRecorder()
	svc.findNearby(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var result []*models.Location
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if result == nil || len(result) != 0 {
		t.Fatalf("expected empty array, got %v", result)
	}
}
