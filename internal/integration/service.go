package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/waydxd/Orbit-core/internal/integration/formats"
	"github.com/waydxd/Orbit-core/internal/integration/google"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
	"github.com/waydxd/Orbit-core/pkg/middleware"
)

// CalendarServiceInterface defines the methods needed from calendar service
type CalendarServiceInterface interface {
	ListEventsAdapter(ctx context.Context, startTime, endTime int64, status string) ([]interface{}, error)
	CreateEventAdapter(ctx context.Context, event interface{}) (interface{}, error)
	UpdateEventAdapter(ctx context.Context, id string, event interface{}) (interface{}, error)
	DeleteEventAdapter(ctx context.Context, id string) error
}

// Service represents the Integration Service
type Service struct {
	config          *config.Config
	logger          *logger.Logger
	calendarService CalendarServiceInterface
	googleService   *google.Service
}

// NewService creates a new Integration Service
func NewService(cfg *config.Config, log *logger.Logger) *Service {
	// Initialize Google Calendar service with in-memory token store.
	// WARNING: The in-memory token store is not suitable for production use because
	// tokens are lost on server restart and cannot be shared across multiple instances.
	// Replace google.NewInMemoryTokenStore with a database-backed implementation
	// before deploying this service to a production environment.
	tokenStore := google.NewInMemoryTokenStore()
	googleSvc := google.NewService(cfg, log, tokenStore)

	return &Service{
		config:        cfg,
		logger:        log,
		googleService: googleSvc,
	}
}

// SetCalendarService sets the calendar service reference for import/export operations
func (s *Service) SetCalendarService(calSvc CalendarServiceInterface) {
	s.calendarService = calSvc
	s.googleService.SetCalendarService(calSvc)
}

// respondWithJSON helper for writing JSON responses
func (s *Service) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if payload != nil {
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			s.logger.Error("failed to write response", "error", err)
		}
	}
}

// respondWithError helper for writing error responses
func (s *Service) respondWithError(w http.ResponseWriter, code int, message string, err error) {
	if err != nil {
		s.logger.Error(message, "error", err)
	} else {
		s.logger.Error(message)
	}
	s.respondWithJSON(w, code, map[string]string{"error": message})
}

// decodeJSON helper for decoding JSON request bodies
func (s *Service) decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// RegisterRoutes registers integration routes
func (s *Service) RegisterRoutes(router *mux.Router) {
	integrationRouter := router.PathPrefix("/integration").Subrouter()

	// Import/Export routes (Phase 1)
	integrationRouter.HandleFunc("/import", s.importCalendar).Methods("POST")
	integrationRouter.HandleFunc("/export", s.exportCalendar).Methods("GET")

	// Google Calendar routes (Phase 2)
	integrationRouter.HandleFunc("/google/auth", s.googleAuth).Methods("GET")
	integrationRouter.HandleFunc("/google/callback", s.googleCallback).Methods("GET")
	integrationRouter.HandleFunc("/google/disconnect", s.googleDisconnect).Methods("POST")
	integrationRouter.HandleFunc("/google/status", s.googleStatus).Methods("GET")
	integrationRouter.HandleFunc("/google/sync", s.googleSync).Methods("POST")
	integrationRouter.HandleFunc("/google/webhook", s.googleWebhook).Methods("POST")
	integrationRouter.HandleFunc("/google/watch", s.googleWatch).Methods("POST")

	// Existing routes
	integrationRouter.HandleFunc("/sync", s.syncData).Methods("POST")
	integrationRouter.HandleFunc("/webhooks", s.handleWebhook).Methods("POST")
	integrationRouter.HandleFunc("/external/connect", s.connectExternal).Methods("POST")
	integrationRouter.HandleFunc("/external/disconnect", s.disconnectExternal).Methods("POST")
	integrationRouter.HandleFunc("/external/status", s.getExternalStatus).Methods("GET")
}

// SyncRequest represents a data synchronization request
type SyncRequest struct {
	Source string                 `json:"source"`
	Target string                 `json:"target"`
	Data   map[string]interface{} `json:"data"`
}

// syncData handles data synchronization between external APIs
func (s *Service) syncData(w http.ResponseWriter, r *http.Request) {
	var req SyncRequest
	if err := s.decodeJSON(r, &req); err != nil {
		s.respondWithError(w, http.StatusBadRequest, "invalid request", err)
		return
	}

	// TODO: Implement data synchronization logic
	s.logger.Info("Data sync initiated", "source", req.Source, "target", req.Target)

	s.respondWithJSON(w, http.StatusOK, map[string]string{"message": "sync completed"})
}

// handleWebhook processes incoming webhooks from external services
func (s *Service) handleWebhook(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := s.decodeJSON(r, &payload); err != nil {
		s.respondWithError(w, http.StatusBadRequest, "invalid webhook payload", err)
		return
	}

	// TODO: Process webhook based on source
	s.logger.Info("Webhook received", "payload", payload)

	s.respondWithJSON(w, http.StatusOK, map[string]string{"message": "webhook processed"})
}

// connectExternal connects to an external API
func (s *Service) connectExternal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Service string `json:"service"`
		APIKey  string `json:"api_key"`
	}

	if err := s.decodeJSON(r, &req); err != nil {
		s.respondWithError(w, http.StatusBadRequest, "invalid request", err)
		return
	}

	// TODO: Store external API credentials securely
	s.logger.Info("External service connected", "service", req.Service)

	s.respondWithJSON(w, http.StatusOK, map[string]string{"message": "external service connected"})
}

// disconnectExternal disconnects from an external API
func (s *Service) disconnectExternal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Service string `json:"service"`
	}

	if err := s.decodeJSON(r, &req); err != nil {
		s.respondWithError(w, http.StatusBadRequest, "invalid request", err)
		return
	}

	// TODO: Remove external API credentials
	s.logger.Info("External service disconnected", "service", req.Service)

	s.respondWithJSON(w, http.StatusOK, map[string]string{"message": "external service disconnected"})
}

// getExternalStatus retrieves status of external integrations
func (s *Service) getExternalStatus(w http.ResponseWriter, _ *http.Request) {
	// TODO: Fetch integration status from database
	status := map[string]interface{}{
		"integrations": []map[string]interface{}{},
	}

	s.respondWithJSON(w, http.StatusOK, status)
}

// ===== Calendar Import/Export Handlers =====

// importCalendar handles importing calendar data from ICS or CSV files
// POST /api/v1/integration/import
// Content-Type: multipart/form-data
func (s *Service) importCalendar(w http.ResponseWriter, r *http.Request) {
	if s.calendarService == nil {
		s.respondWithError(w, http.StatusServiceUnavailable, "calendar service not available", nil)
		return
	}

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		s.respondWithError(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	// Parse multipart form with a 50MB max size
	const maxFileSize = 50 << 20 // 50MB
	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		s.respondWithError(w, http.StatusBadRequest, "invalid multipart form", err)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.respondWithError(w, http.StatusBadRequest, "file is required", err)
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			s.logger.Error("failed to close uploaded file", "error", err)
		}
	}(file)

	// Determine file format from extension or Content-Type
	filename := header.Filename
	ext := strings.ToLower(filepath.Ext(filename))
	contentType := header.Header.Get("Content-Type")

	var events []*models.Event
	var parseErr error

	switch {
	case ext == ".ics" || strings.Contains(contentType, "text/calendar"):
		events, parseErr = formats.ParseICS(file, userID)
	case ext == ".csv" || strings.Contains(contentType, "text/csv"):
		events, parseErr = formats.ParseCSV(file, userID)
	default:
		s.respondWithError(w, http.StatusBadRequest, "unsupported file format. Supported formats: .ics, .csv", nil)
		return
	}

	if parseErr != nil {
		s.respondWithError(w, http.StatusBadRequest, "failed to parse file", parseErr)
		return
	}

	// Import events to calendar
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	importedCount := 0
	var importErrors []string

	for _, event := range events {
		_, err := s.calendarService.CreateEventAdapter(ctx, event)
		if err != nil {
			s.logger.Error("failed to import event", "title", event.Title, "error", err)
			importErrors = append(importErrors, fmt.Sprintf("Failed to import '%s': %v", event.Title, err))
		} else {
			importedCount++
		}
	}

	s.logger.Info("Calendar import completed",
		"user_id", userID,
		"total_events", len(events),
		"imported", importedCount,
		"errors", len(importErrors))

	response := map[string]interface{}{
		"message":        "import completed",
		"total_events":   len(events),
		"imported_count": importedCount,
	}
	if len(importErrors) > 0 {
		response["errors"] = importErrors
	}

	s.respondWithJSON(w, http.StatusOK, response)
}

// exportCalendar handles exporting calendar data to ICS or CSV format
// GET /api/v1/integration/export
// Query params: format (optional, default: ics), start_time, end_time
func (s *Service) exportCalendar(w http.ResponseWriter, r *http.Request) {
	// Check service availability
	if s.calendarService == nil {
		s.respondWithError(w, http.StatusServiceUnavailable, "calendar service not available", nil)
		return
	}

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		s.respondWithError(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	// Parse and validate parameters
	format, startTime, endTime, status, err := s.parseExportParams(r)
	if err != nil {
		s.respondWithError(w, status, err.Error(), nil)
		return
	}

	// Fetch and filter events
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	events, err := s.fetchEventsFiltered(ctx, startTime, endTime, userID)
	if err != nil {
		s.respondWithError(w, http.StatusInternalServerError, "failed to fetch events", err)
		return
	}

	// Generate export data
	data, contentType, filename, err := s.generateExportData(format, events)
	if err != nil {
		s.respondWithError(w, http.StatusInternalServerError, "failed to generate export", err)
		return
	}

	s.logger.Info("Calendar export completed",
		"user_id", userID,
		"format", format,
		"events_count", len(events))

	// Set response headers for file download
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// parseExportParams parses and validates export query parameters.
// Returns format, startTime, endTime, httpStatus (non-0 when err!=nil), and error.
func (s *Service) parseExportParams(r *http.Request) (string, int64, int64, int, error) {
	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "" {
		format = "ics"
	}
	if format != "ics" && format != "csv" {
		return "", 0, 0, http.StatusBadRequest, fmt.Errorf("unsupported format. Supported formats: ics, csv")
	}

	now := time.Now()
	startTime := now.AddDate(-1, 0, 0).Unix() // 1 year ago
	endTime := now.AddDate(1, 0, 0).Unix()    // 1 year from now

	if startTimeStr := r.URL.Query().Get("start_time"); startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = t.Unix()
		}
	}
	if endTimeStr := r.URL.Query().Get("end_time"); endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			endTime = t.Unix()
		}
	}

	return format, startTime, endTime, 0, nil
}

// fetchEventsFiltered retrieves events from the calendar service and filters them by userID.
func (s *Service) fetchEventsFiltered(ctx context.Context, startTime, endTime int64, userID string) ([]*models.Event, error) {
	eventsInterface, err := s.calendarService.ListEventsAdapter(ctx, startTime, endTime, "")
	if err != nil {
		return nil, err
	}

	var events []*models.Event
	for _, e := range eventsInterface {
		if event, ok := e.(*models.Event); ok {
			if event.UserID == userID {
				events = append(events, event)
			}
		}
	}
	return events, nil
}

// generateExportData produces export bytes, content-type and filename based on format.
func (s *Service) generateExportData(format string, events []*models.Event) ([]byte, string, string, error) {
	switch format {
	case "ics":
		data, err := formats.GenerateICS(events)
		return data, "text/calendar; charset=utf-8", "calendar_export.ics", err
	case "csv":
		data, err := formats.GenerateCSV(events)
		return data, "text/csv; charset=utf-8", "calendar_export.csv", err
	default:
		return nil, "", "", fmt.Errorf("unsupported format: %s", format)
	}
}

// ===== Google Calendar Integration Handlers (Phase 2) =====

// googleAuth initiates the Google OAuth flow
// GET /api/v1/integration/google/auth
func (s *Service) googleAuth(w http.ResponseWriter, r *http.Request) {
	if !s.googleService.IsConfigured() {
		s.respondWithError(w, http.StatusServiceUnavailable, "Google Calendar integration is not configured", nil)
		return
	}

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		s.respondWithError(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	authURL, err := s.googleService.GetAuthURL(userID)
	if err != nil {
		s.respondWithError(w, http.StatusInternalServerError, "failed to generate authorization URL", err)
		return
	}

	s.respondWithJSON(w, http.StatusOK, map[string]string{
		"auth_url": authURL,
		"message":  "Redirect the user to auth_url to authorize Google Calendar access",
	})
}

// googleCallback handles the OAuth callback from Google
// GET /api/v1/integration/google/callback?state=xxx&code=xxx
func (s *Service) googleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	errorParam := r.URL.Query().Get("error")

	// Handle OAuth errors
	if errorParam != "" {
		s.logger.Error("OAuth error from Google", "error", errorParam)
		http.Error(w, "Authorization denied: "+errorParam, http.StatusBadRequest)
		return
	}

	if state == "" || code == "" {
		http.Error(w, "Missing state or code parameter", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	userID, err := s.googleService.HandleCallback(ctx, state, code)
	if err != nil {
		s.logger.Error("OAuth callback failed", "error", err)
		http.Error(w, "Authorization failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	s.respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Google Calendar connected successfully",
		"user_id": userID,
	})
}

// googleDisconnect disconnects Google Calendar for a user
// POST /api/v1/integration/google/disconnect
func (s *Service) googleDisconnect(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		s.respondWithError(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := s.googleService.Disconnect(ctx, userID); err != nil {
		s.respondWithError(w, http.StatusInternalServerError, "failed to disconnect", err)
		return
	}

	s.respondWithJSON(w, http.StatusOK, map[string]string{"message": "Google Calendar disconnected successfully"})
}

// googleStatus returns the Google Calendar connection status for a user
// GET /api/v1/integration/google/status
func (s *Service) googleStatus(w http.ResponseWriter, r *http.Request) {
	if !s.googleService.IsConfigured() {
		s.respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"configured": false,
			"connected":  false,
			"message":    "Google Calendar integration is not configured",
		})
		return
	}

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		s.respondWithError(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	status, err := s.googleService.GetConnectionStatus(ctx, userID)
	if err != nil {
		s.respondWithError(w, http.StatusInternalServerError, "failed to get status", err)
		return
	}

	status["configured"] = true
	s.respondWithJSON(w, http.StatusOK, status)
}

// GoogleSyncRequest represents a Google Calendar sync request
type GoogleSyncRequest struct {
	Direction string `json:"direction"`  // "from_google", "to_google", or "bidirectional"
	StartTime string `json:"start_time"` // optional, RFC3339 format
	EndTime   string `json:"end_time"`   // optional, RFC3339 format
}

// googleSync triggers a sync between local calendar and Google Calendar
// POST /api/v1/integration/google/sync
func (s *Service) googleSync(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		s.respondWithError(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	var req GoogleSyncRequest
	if err := s.decodeJSON(r, &req); err != nil {
		s.respondWithError(w, http.StatusBadRequest, "invalid request", err)
		return
	}

	startTime, endTime := s.parseSyncTimeRange(req.StartTime, req.EndTime)

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	result, err := s.executeGoogleSync(ctx, userID, req.Direction, startTime, endTime)
	if err != nil {
		// executeGoogleSync already handles specific errors, but we wrap it here if needed
		s.respondWithError(w, http.StatusInternalServerError, "sync failed", err)
		return
	}

	s.respondWithJSON(w, http.StatusOK, result)
}

// parseSyncTimeRange parses start and end times or returns defaults
func (s *Service) parseSyncTimeRange(startStr, endStr string) (time.Time, time.Time) {
	now := time.Now()
	startTime := now.AddDate(0, -1, 0) // 1 month ago
	endTime := now.AddDate(0, 1, 0)    // 1 month from now

	if startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			startTime = t
		}
	}
	if endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			endTime = t
		}
	}

	return startTime, endTime
}

// executeGoogleSync runs the requested sync logic
func (s *Service) executeGoogleSync(ctx context.Context, userID, direction string, startTime, endTime time.Time) (map[string]interface{}, error) {
	if direction == "" {
		direction = "from_google"
	}

	result := map[string]interface{}{
		"direction": direction,
	}

	switch direction {
	case "from_google":
		count, err := s.googleService.SyncFromGoogle(ctx, userID, startTime, endTime)
		if err != nil {
			return nil, fmt.Errorf("sync from Google failed: %w", err)
		}
		result["imported_count"] = count
		result["message"] = "Sync from Google Calendar completed"

	case "to_google":
		count, err := s.googleService.SyncToGoogle(ctx, userID, startTime, endTime)
		if err != nil {
			return nil, fmt.Errorf("sync to Google failed: %w", err)
		}
		result["exported_count"] = count
		result["message"] = "Sync to Google Calendar completed"

	case "bidirectional":
		return s.performBidirectionalSync(ctx, userID, startTime, endTime), nil

	default:
		return nil, fmt.Errorf("invalid direction. Use 'from_google', 'to_google', or 'bidirectional'")
	}

	return result, nil
}

// performBidirectionalSync handles two-way synchronization
func (s *Service) performBidirectionalSync(ctx context.Context, userID string, startTime, endTime time.Time) map[string]interface{} {
	result := map[string]interface{}{
		"direction": "bidirectional",
	}

	importCount, importErr := s.googleService.SyncFromGoogle(ctx, userID, startTime, endTime)
	if importErr != nil {
		s.logger.Warn("sync from Google failed during bidirectional sync", "error", importErr, "user_id", userID)
		result["from_google_error"] = importErr.Error()
	}
	result["imported_count"] = importCount

	exportCount, exportErr := s.googleService.SyncToGoogle(ctx, userID, startTime, endTime)
	if exportErr != nil {
		s.logger.Warn("sync to Google failed during bidirectional sync", "error", exportErr, "user_id", userID)
		result["to_google_error"] = exportErr.Error()
	}
	result["exported_count"] = exportCount

	if importErr != nil || exportErr != nil {
		result["message"] = "Bidirectional sync completed with errors"
	} else {
		result["message"] = "Bidirectional sync completed"
	}

	return result
}

// googleWebhook handles incoming push notifications from Google Calendar
// POST /api/v1/integration/google/webhook
func (s *Service) googleWebhook(w http.ResponseWriter, r *http.Request) {
	_, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := s.googleService.HandleWebhook(r); err != nil {
		s.logger.Error("failed to handle Google webhook", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Google expects a 200 OK response
	w.WriteHeader(http.StatusOK)
}

// googleWatch sets up a watch channel for Google Calendar push notifications
// POST /api/v1/integration/google/watch
func (s *Service) googleWatch(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		s.respondWithError(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	channel, err := s.googleService.SetupWatch(ctx, userID)
	if err != nil {
		s.respondWithError(w, http.StatusInternalServerError, "failed to setup watch", err)
		return
	}

	s.respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"message":     "Watch channel created successfully",
		"channel_id":  channel.Id,
		"resource_id": channel.ResourceId,
		"expiration":  channel.Expiration,
	})
}
