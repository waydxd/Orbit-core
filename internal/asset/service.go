package asset

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/waydxd/Orbit-core/internal/shared/database"
	dbq "github.com/waydxd/Orbit-core/internal/shared/database/db"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
	"github.com/waydxd/Orbit-core/pkg/middleware"
)

const (
	maxFileSize    = 5 * 1024 * 1024 // 5 MB
	maxEventImages = 5
)

// allowedMIMETypes maps valid MIME types to their magic-byte prefixes.
var allowedMIMETypes = map[string][]byte{
	"image/jpeg": {0xFF, 0xD8, 0xFF},
	"image/png":  {0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
	"image/webp": {0x52, 0x49, 0x46, 0x46}, // followed by 4 size bytes then "WEBP"
}

// Service handles HTTP routes for binary asset upload and serving.
type Service struct {
	config  *config.Config
	logger  *logger.Logger
	repo    Repository
	queries assetQueries
}

type assetQueries interface {
	GetEventByID(ctx context.Context, id pgtype.UUID) (dbq.GetEventByIDRow, error)
	GetEventImageURLs(ctx context.Context, id pgtype.UUID) ([]string, error)
	AddEventImageURL(ctx context.Context, arg dbq.AddEventImageURLParams) error
	UpdateUserProfilePicURL(ctx context.Context, arg dbq.UpdateUserProfilePicURLParams) error
}

// NewService creates a new asset Service.
func NewService(cfg *config.Config, log *logger.Logger, repo Repository, db *database.DB) *Service {
	return &Service{
		config:  cfg,
		logger:  log,
		repo:    repo,
		queries: dbq.New(db.Pool),
	}
}

// RegisterRoutes registers asset routes on the provided (protected) router.
func (s *Service) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/events/{id}/images", s.uploadEventImage).Methods(http.MethodPost)
	r.HandleFunc("/users/me/profile-pic", s.uploadProfilePic).Methods(http.MethodPost)
	r.HandleFunc("/assets/events/{image_id}", s.serveEventImage).Methods(http.MethodGet)
	r.HandleFunc("/assets/users/{image_id}", s.serveUserAvatar).Methods(http.MethodGet)
}

// uploadEventImage handles POST /events/{id}/images
func (s *Service) uploadEventImage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		s.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	eventID := mux.Vars(r)["id"]

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Verify event exists and belongs to the user.
	event, err := s.queries.GetEventByID(ctx, database.StringToUUID(eventID))
	if err != nil {
		s.writeError(w, http.StatusNotFound, "event not found")
		return
	}
	if database.UUIDToString(event.UserID) != userID {
		s.writeError(w, http.StatusForbidden, "access denied")
		return
	}

	// Check current image count.
	existingURLs, err := s.queries.GetEventImageURLs(ctx, database.StringToUUID(eventID))
	if err != nil {
		s.logger.Error("Failed to get event image URLs", "error", err)
		s.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if len(existingURLs) >= maxEventImages {
		s.writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("event already has %d images (maximum)", maxEventImages))
		return
	}

	data, contentType, err := readAndValidateImage(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	imageID, err := s.repo.SaveEventImage(ctx, eventID, data, contentType)
	if err != nil {
		s.logger.Error("Failed to save event image", "error", err)
		s.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	imageURL := fmt.Sprintf("/api/v1/assets/events/%s", imageID)
	if err := s.queries.AddEventImageURL(ctx, dbq.AddEventImageURLParams{
		Url:       imageURL,
		UpdatedAt: database.TimeToTimestamptz(time.Now().UTC()),
		ID:        database.StringToUUID(eventID),
	}); err != nil {
		s.logger.Error("Failed to update event image URL in PostgreSQL", "error", err)
		// Attempt to clean up the uploaded image.
		if cleanupErr := s.repo.DeleteEventImage(ctx, imageID); cleanupErr != nil {
			s.logger.Warn("Failed to clean up uploaded event image after PostgreSQL error", "image_id", imageID, "error", cleanupErr)
		}
		s.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"image_id": imageID, "url": imageURL})
}

// uploadProfilePic handles POST /users/me/profile-pic
func (s *Service) uploadProfilePic(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		s.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	data, contentType, err := readAndValidateImage(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	imageID, err := s.repo.SaveUserAvatar(ctx, userID, data, contentType)
	if err != nil {
		s.logger.Error("Failed to save user avatar", "error", err)
		s.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	imageURL := fmt.Sprintf("/api/v1/assets/users/%s", imageID)
	if err := s.queries.UpdateUserProfilePicURL(ctx, dbq.UpdateUserProfilePicURLParams{
		ProfilePicUrl: database.StringToText(imageURL),
		UpdatedAt:     database.TimeToTimestamptz(time.Now().UTC()),
		ID:            database.StringToUUID(userID),
	}); err != nil {
		s.logger.Error("Failed to update user profile_pic_url in PostgreSQL", "error", err)
		s.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"image_id": imageID, "url": imageURL})
}

// serveEventImage handles GET /assets/events/{image_id}
func (s *Service) serveEventImage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		s.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	imageID := mux.Vars(r)["image_id"]

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	img, err := s.repo.GetEventImage(ctx, imageID)
	if err != nil {
		if errors.Is(err, ErrAssetNotFound) {
			s.writeError(w, http.StatusNotFound, "image not found")
			return
		}
		s.logger.Error("Failed to retrieve event image", "error", err)
		s.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	event, err := s.queries.GetEventByID(ctx, database.StringToUUID(img.Metadata.EventID))
	if err != nil || database.UUIDToString(event.UserID) != userID {
		s.writeError(w, http.StatusNotFound, "image not found")
		return
	}

	w.Header().Set("Content-Type", img.Metadata.ContentType)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, bytes.NewReader(img.BinData))
}

// serveUserAvatar handles GET /assets/users/{image_id}
func (s *Service) serveUserAvatar(w http.ResponseWriter, r *http.Request) {
	imageID := mux.Vars(r)["image_id"]

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	av, err := s.repo.GetUserAvatar(ctx, imageID)
	if err != nil {
		if errors.Is(err, ErrAssetNotFound) {
			s.writeError(w, http.StatusNotFound, "image not found")
			return
		}
		s.logger.Error("Failed to retrieve user avatar", "error", err)
		s.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Content-Type", av.Metadata.ContentType)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, bytes.NewReader(av.BinData))
}

// readAndValidateImage reads the multipart image from the request and validates it.
// It returns the raw bytes and the detected MIME type.
func readAndValidateImage(r *http.Request) ([]byte, string, error) {
	const multipartBodyLimit = maxFileSize + 1024

	r.Body = http.MaxBytesReader(nil, r.Body, multipartBodyLimit)
	if err := r.ParseMultipartForm(multipartBodyLimit); err != nil {
		return nil, "", errors.New("invalid multipart form")
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		return nil, "", errors.New("field 'image' is required")
	}
	defer file.Close()

	// Read up to maxFileSize+1 bytes so we can detect over-limit uploads.
	data, err := io.ReadAll(io.LimitReader(file, maxFileSize+1))
	if err != nil {
		return nil, "", errors.New("failed to read image data")
	}
	if int64(len(data)) > maxFileSize {
		return nil, "", fmt.Errorf("file exceeds maximum size of %d MB", maxFileSize/(1024*1024))
	}
	if len(data) == 0 {
		return nil, "", errors.New("image file is empty")
	}

	contentType, err := detectContentType(data)
	if err != nil {
		return nil, "", err
	}

	return data, contentType, nil
}

// detectContentType validates the image's magic bytes and returns the MIME type.
func detectContentType(data []byte) (string, error) {
	for mimeType, magic := range allowedMIMETypes {
		if len(data) >= len(magic) && bytes.Equal(data[:len(magic)], magic) {
			// Extra check for WebP: bytes 8-11 must be "WEBP".
			if mimeType == "image/webp" {
				if len(data) < 12 || string(data[8:12]) != "WEBP" {
					continue
				}
			}
			return mimeType, nil
		}
	}
	return "", errors.New("unsupported image type: allowed types are JPEG, PNG, and WebP")
}

// writeError writes a JSON error response.
func (s *Service) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
