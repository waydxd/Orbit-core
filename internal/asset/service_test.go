package asset

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/waydxd/Orbit-core/internal/shared/database"
	dbq "github.com/waydxd/Orbit-core/internal/shared/database/db"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/logger"
	"github.com/waydxd/Orbit-core/pkg/middleware"
)

// ===== mock assetQueries =====

type mockQueries struct {
	eventRow       dbq.GetEventByIDRow
	eventErr       error
	imageURLs      []string
	imageURLsErr   error
	addImageRows   int64
	addImageErr    error
	updatePicErr   error
}

func (m *mockQueries) GetEventByID(_ context.Context, _ pgtype.UUID) (dbq.GetEventByIDRow, error) {
	return m.eventRow, m.eventErr
}

func (m *mockQueries) GetEventImageURLs(_ context.Context, _ pgtype.UUID) ([]string, error) {
	return m.imageURLs, m.imageURLsErr
}

func (m *mockQueries) AddEventImageURLIfCapacity(_ context.Context, _ dbq.AddEventImageURLIfCapacityParams) (int64, error) {
	return m.addImageRows, m.addImageErr
}

func (m *mockQueries) UpdateUserProfilePicURL(_ context.Context, _ dbq.UpdateUserProfilePicURLParams) error {
	return m.updatePicErr
}

// ===== mock Repository =====

type mockAssetRepo struct {
	saveEventImageID  string
	saveEventImageErr error
	getEventImageResp *models.EventImage
	getEventImageErr  error
	deleteEventImgErr error
	saveAvatarID      string
	saveAvatarErr     error
	getAvatarResp     *models.UserAvatar
	getAvatarErr      error
	deleteAvatarErr   error
}

func (m *mockAssetRepo) SaveEventImage(_ context.Context, _ string, _ []byte, _ string) (string, error) {
	return m.saveEventImageID, m.saveEventImageErr
}

func (m *mockAssetRepo) GetEventImage(_ context.Context, _ string) (*models.EventImage, error) {
	return m.getEventImageResp, m.getEventImageErr
}

func (m *mockAssetRepo) DeleteEventImage(_ context.Context, _ string) error {
	return m.deleteEventImgErr
}

func (m *mockAssetRepo) SaveUserAvatar(_ context.Context, _ string, _ []byte, _ string) (string, error) {
	return m.saveAvatarID, m.saveAvatarErr
}

func (m *mockAssetRepo) DeleteUserAvatar(_ context.Context, _ string) error {
	return m.deleteAvatarErr
}

func (m *mockAssetRepo) GetUserAvatar(_ context.Context, _ string) (*models.UserAvatar, error) {
	return m.getAvatarResp, m.getAvatarErr
}

// ===== helpers =====

func newTestAssetService(repo Repository, q assetQueries) *Service {
	return &Service{
		logger:  logger.New(),
		repo:    repo,
		queries: q,
	}
}

func withUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
	return r.WithContext(ctx)
}

const (
	testUserID  = "11111111-1111-1111-1111-111111111111"
	testUser2ID = "22222222-2222-2222-2222-222222222222"
	testEventID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	testImageID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
)

func makeEventID(userID string) dbq.GetEventByIDRow {
	row := dbq.GetEventByIDRow{}
	row.UserID = database.StringToUUID(userID)
	return row
}

// buildMultipartRequest creates a multipart/form-data request with an "image" field.
func buildMultipartRequest(t *testing.T, method, url string, data []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("image", "test.jpg")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatalf("failed to write form file data: %v", err)
	}
	w.Close()

	req := httptest.NewRequest(method, url, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

// ===== detectContentType tests =====

func TestDetectContentType_JPEG(t *testing.T) {
	data := []byte{0xFF, 0xD8, 0xFF, 0x00, 0x00, 0x00}
	ct, err := detectContentType(data)
	if err != nil {
		t.Fatalf("expected JPEG detection, got error: %v", err)
	}
	if ct != "image/jpeg" {
		t.Fatalf("expected image/jpeg, got %q", ct)
	}
}

func TestDetectContentType_PNG(t *testing.T) {
	data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00}
	ct, err := detectContentType(data)
	if err != nil {
		t.Fatalf("expected PNG detection, got error: %v", err)
	}
	if ct != "image/png" {
		t.Fatalf("expected image/png, got %q", ct)
	}
}

func TestDetectContentType_WebP(t *testing.T) {
	// RIFF + 4 size bytes + "WEBP"
	data := []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P', 0x00}
	ct, err := detectContentType(data)
	if err != nil {
		t.Fatalf("expected WebP detection, got error: %v", err)
	}
	if ct != "image/webp" {
		t.Fatalf("expected image/webp, got %q", ct)
	}
}

func TestDetectContentType_WebP_MissingMarker(t *testing.T) {
	// RIFF magic but bytes 8-11 are not "WEBP"
	data := []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 'A', 'V', 'I', ' ', 0x00}
	_, err := detectContentType(data)
	if err == nil {
		t.Fatal("expected error for RIFF without WEBP marker")
	}
}

func TestDetectContentType_Unsupported(t *testing.T) {
	data := []byte{0x00, 0x01, 0x02, 0x03}
	_, err := detectContentType(data)
	if err == nil {
		t.Fatal("expected error for unsupported image type")
	}
}

func TestDetectContentType_Empty(t *testing.T) {
	_, err := detectContentType([]byte{})
	if err == nil {
		t.Fatal("expected error for empty data")
	}
}

// ===== uploadEventImage handler tests =====

func TestUploadEventImage_Unauthorized(t *testing.T) {
	svc := newTestAssetService(&mockAssetRepo{}, &mockQueries{})
	req := httptest.NewRequest(http.MethodPost, "/events/evt-1/images", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "evt-1"})
	rr := httptest.NewRecorder()
	svc.uploadEventImage(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestUploadEventImage_EventNotFound(t *testing.T) {
	q := &mockQueries{eventErr: errors.New("not found")}
	svc := newTestAssetService(&mockAssetRepo{}, q)

	req := buildMultipartRequest(t, http.MethodPost, "/events/"+testEventID+"/images", jpegBytes())
	req = withUserID(req, testUserID)
	req = mux.SetURLVars(req, map[string]string{"id": testEventID})
	rr := httptest.NewRecorder()
	svc.uploadEventImage(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestUploadEventImage_Forbidden(t *testing.T) {
	// Event belongs to user-2, but request is from user-1
	q := &mockQueries{eventRow: makeEventID(testUser2ID)}
	svc := newTestAssetService(&mockAssetRepo{}, q)

	req := buildMultipartRequest(t, http.MethodPost, "/events/"+testEventID+"/images", jpegBytes())
	req = withUserID(req, testUserID)
	req = mux.SetURLVars(req, map[string]string{"id": testEventID})
	rr := httptest.NewRecorder()
	svc.uploadEventImage(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestUploadEventImage_NoImageField(t *testing.T) {
	q := &mockQueries{eventRow: makeEventID(testUserID)}
	svc := newTestAssetService(&mockAssetRepo{}, q)

	// Send a request without the "image" field
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("other_field", "value")
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/events/"+testEventID+"/images", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req = withUserID(req, testUserID)
	req = mux.SetURLVars(req, map[string]string{"id": testEventID})
	rr := httptest.NewRecorder()
	svc.uploadEventImage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when image field missing, got %d", rr.Code)
	}
}

func TestUploadEventImage_RepoSaveError(t *testing.T) {
	q := &mockQueries{eventRow: makeEventID(testUserID)}
	repo := &mockAssetRepo{saveEventImageErr: errors.New("mongo error")}
	svc := newTestAssetService(repo, q)

	req := buildMultipartRequest(t, http.MethodPost, "/events/"+testEventID+"/images", jpegBytes())
	req = withUserID(req, testUserID)
	req = mux.SetURLVars(req, map[string]string{"id": testEventID})
	rr := httptest.NewRecorder()
	svc.uploadEventImage(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestUploadEventImage_CapacityExceeded(t *testing.T) {
	q := &mockQueries{
		eventRow:     makeEventID(testUserID),
		addImageRows: 0, // 0 means capacity exceeded
	}
	repo := &mockAssetRepo{saveEventImageID: testImageID}
	svc := newTestAssetService(repo, q)

	req := buildMultipartRequest(t, http.MethodPost, "/events/"+testEventID+"/images", jpegBytes())
	req = withUserID(req, testUserID)
	req = mux.SetURLVars(req, map[string]string{"id": testEventID})
	rr := httptest.NewRecorder()
	svc.uploadEventImage(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 when capacity exceeded, got %d", rr.Code)
	}
}

func TestUploadEventImage_Success(t *testing.T) {
	q := &mockQueries{
		eventRow:     makeEventID(testUserID),
		addImageRows: 1,
	}
	repo := &mockAssetRepo{saveEventImageID: testImageID}
	svc := newTestAssetService(repo, q)

	req := buildMultipartRequest(t, http.MethodPost, "/events/"+testEventID+"/images", jpegBytes())
	req = withUserID(req, testUserID)
	req = mux.SetURLVars(req, map[string]string{"id": testEventID})
	rr := httptest.NewRecorder()
	svc.uploadEventImage(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("expected JSON response: %v", err)
	}
	if resp["image_id"] != testImageID {
		t.Fatalf("expected image_id=%s, got %q", testImageID, resp["image_id"])
	}
	if resp["url"] != fmt.Sprintf("/api/v1/assets/events/%s", testImageID) {
		t.Fatalf("unexpected url: %q", resp["url"])
	}
}

// ===== listEventImages handler tests =====

func TestListEventImages_Unauthorized(t *testing.T) {
	svc := newTestAssetService(&mockAssetRepo{}, &mockQueries{})
	req := httptest.NewRequest(http.MethodGet, "/events/evt-1/images", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "evt-1"})
	rr := httptest.NewRecorder()
	svc.listEventImages(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestListEventImages_EventNotFound(t *testing.T) {
	q := &mockQueries{eventErr: errors.New("not found")}
	svc := newTestAssetService(&mockAssetRepo{}, q)

	req := httptest.NewRequest(http.MethodGet, "/events/"+testEventID+"/images", nil)
	req = withUserID(req, testUserID)
	req = mux.SetURLVars(req, map[string]string{"id": testEventID})
	rr := httptest.NewRecorder()
	svc.listEventImages(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestListEventImages_Success(t *testing.T) {
	q := &mockQueries{
		eventRow:  makeEventID(testUserID),
		imageURLs: []string{"/api/v1/assets/events/img-1"},
	}
	svc := newTestAssetService(&mockAssetRepo{}, q)

	req := httptest.NewRequest(http.MethodGet, "/events/"+testEventID+"/images", nil)
	req = withUserID(req, testUserID)
	req = mux.SetURLVars(req, map[string]string{"id": testEventID})
	rr := httptest.NewRecorder()
	svc.listEventImages(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string][]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("expected JSON response: %v", err)
	}
	if len(resp["images"]) != 1 {
		t.Fatalf("expected 1 image URL, got %d", len(resp["images"]))
	}
}

// ===== uploadProfilePic handler tests =====

func TestUploadProfilePic_Unauthorized(t *testing.T) {
	svc := newTestAssetService(&mockAssetRepo{}, &mockQueries{})
	req := httptest.NewRequest(http.MethodPost, "/users/me/profile-pic", nil)
	rr := httptest.NewRecorder()
	svc.uploadProfilePic(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestUploadProfilePic_RepoError(t *testing.T) {
	repo := &mockAssetRepo{saveAvatarErr: errors.New("mongo error")}
	svc := newTestAssetService(repo, &mockQueries{})

	req := buildMultipartRequest(t, http.MethodPost, "/users/me/profile-pic", jpegBytes())
	req = withUserID(req, "user-1")
	rr := httptest.NewRecorder()
	svc.uploadProfilePic(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestUploadProfilePic_DBError(t *testing.T) {
	repo := &mockAssetRepo{saveAvatarID: "av-1"}
	q := &mockQueries{updatePicErr: errors.New("db error")}
	svc := newTestAssetService(repo, q)

	req := buildMultipartRequest(t, http.MethodPost, "/users/me/profile-pic", jpegBytes())
	req = withUserID(req, "user-1")
	rr := httptest.NewRecorder()
	svc.uploadProfilePic(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestUploadProfilePic_Success(t *testing.T) {
	repo := &mockAssetRepo{saveAvatarID: "av-created"}
	svc := newTestAssetService(repo, &mockQueries{})

	req := buildMultipartRequest(t, http.MethodPost, "/users/me/profile-pic", jpegBytes())
	req = withUserID(req, "user-1")
	rr := httptest.NewRecorder()
	svc.uploadProfilePic(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("expected JSON response: %v", err)
	}
	if resp["image_id"] != "av-created" {
		t.Fatalf("expected image_id=av-created, got %q", resp["image_id"])
	}
}

// ===== serveEventImage handler tests =====

func TestServeEventImage_Unauthorized(t *testing.T) {
	svc := newTestAssetService(&mockAssetRepo{}, &mockQueries{})
	req := httptest.NewRequest(http.MethodGet, "/assets/events/img-1", nil)
	req = mux.SetURLVars(req, map[string]string{"image_id": "img-1"})
	rr := httptest.NewRecorder()
	svc.serveEventImage(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestServeEventImage_NotFound(t *testing.T) {
	repo := &mockAssetRepo{getEventImageErr: ErrAssetNotFound}
	svc := newTestAssetService(repo, &mockQueries{})

	req := httptest.NewRequest(http.MethodGet, "/assets/events/img-1", nil)
	req = withUserID(req, "user-1")
	req = mux.SetURLVars(req, map[string]string{"image_id": "img-1"})
	rr := httptest.NewRecorder()
	svc.serveEventImage(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestServeEventImage_Success(t *testing.T) {
	imgData := jpegBytes()
	repo := &mockAssetRepo{
		getEventImageResp: &models.EventImage{
			ID:      testImageID,
			BinData: imgData,
			Metadata: models.EventImageMetadata{
				EventID:     testEventID,
				ContentType: "image/jpeg",
			},
		},
	}
	q := &mockQueries{eventRow: makeEventID(testUserID)}
	svc := newTestAssetService(repo, q)

	req := httptest.NewRequest(http.MethodGet, "/assets/events/"+testImageID, nil)
	req = withUserID(req, testUserID)
	req = mux.SetURLVars(req, map[string]string{"image_id": testImageID})
	rr := httptest.NewRecorder()
	svc.serveEventImage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("expected Content-Type=image/jpeg, got %q", rr.Header().Get("Content-Type"))
	}
	if !bytes.Equal(rr.Body.Bytes(), imgData) {
		t.Fatal("response body did not match image data")
	}
}

// ===== serveUserAvatar handler tests =====

func TestServeUserAvatar_NotFound(t *testing.T) {
	repo := &mockAssetRepo{getAvatarErr: ErrAssetNotFound}
	svc := newTestAssetService(repo, &mockQueries{})

	req := httptest.NewRequest(http.MethodGet, "/assets/users/av-1", nil)
	req = mux.SetURLVars(req, map[string]string{"image_id": "av-1"})
	rr := httptest.NewRecorder()
	svc.serveUserAvatar(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestServeUserAvatar_Success(t *testing.T) {
	imgData := pngBytes()
	repo := &mockAssetRepo{
		getAvatarResp: &models.UserAvatar{
			ID:      "av-1",
			BinData: imgData,
			Metadata: models.UserAvatarMetadata{
				ContentType: "image/png",
			},
		},
	}
	svc := newTestAssetService(repo, &mockQueries{})

	req := httptest.NewRequest(http.MethodGet, "/assets/users/av-1", nil)
	req = mux.SetURLVars(req, map[string]string{"image_id": "av-1"})
	rr := httptest.NewRecorder()
	svc.serveUserAvatar(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("expected Content-Type=image/png, got %q", rr.Header().Get("Content-Type"))
	}
}

// ===== image byte helpers =====

func jpegBytes() []byte {
	return []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
}

func pngBytes() []byte {
	return []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00}
}
