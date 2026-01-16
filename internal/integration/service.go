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
	// Initialize Google Calendar service with in-memory token store
	// TODO: Replace with database-backed token store in production
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
	w.Header().Set("Content-Type", "application/json")

	var req SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("failed to decode sync request", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"}); err != nil {
			s.logger.Error("failed to write sync error response", "error", err)
			return
		}
		return
	}

	// TODO: Implement data synchronization logic
	s.logger.Info("Data sync initiated", "source", req.Source, "target", req.Target)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "sync completed"}); err != nil {
		s.logger.Error("failed to write sync success response", "error", err)
		return
	}
}

// handleWebhook processes incoming webhooks from external services
func (s *Service) handleWebhook(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		s.logger.Error("failed to decode webhook payload", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "invalid webhook payload"}); err != nil {
			s.logger.Error("failed to write webhook error response", "error", err)
			return
		}
		return
	}

	// TODO: Process webhook based on source
	s.logger.Info("Webhook received", "payload", payload)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "webhook processed"}); err != nil {
		s.logger.Error("failed to write webhook success response", "error", err)
		return
	}
}

// connectExternal connects to an external API
func (s *Service) connectExternal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Service string `json:"service"`
		APIKey  string `json:"api_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("failed to decode connect request", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"}); err != nil {
			s.logger.Error("failed to write connect error response", "error", err)
			return
		}
		return
	}

	// TODO: Store external API credentials securely
	s.logger.Info("External service connected", "service", req.Service)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "external service connected"}); err != nil {
		s.logger.Error("failed to write connect success response", "error", err)
		return
	}
}

// disconnectExternal disconnects from an external API
func (s *Service) disconnectExternal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Service string `json:"service"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("failed to decode disconnect request", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"}); err != nil {
			s.logger.Error("failed to write disconnect error response", "error", err)
			return
		}
		return
	}

	// TODO: Remove external API credentials
	s.logger.Info("External service disconnected", "service", req.Service)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "external service disconnected"}); err != nil {
		s.logger.Error("failed to write disconnect success response", "error", err)
		return
	}
}

// getExternalStatus retrieves status of external integrations
func (s *Service) getExternalStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// TODO: Fetch integration status from database
	status := map[string]interface{}{
		"integrations": []map[string]interface{}{},
	}

	if err := json.NewEncoder(w).Encode(status); err != nil {
		s.logger.Error("failed to write external status response", "error", err)
		return
	}
}

// ===== Calendar Import/Export Handlers =====

// importCalendar handles importing calendar data from ICS or CSV files
// POST /api/v1/integration/import
// Content-Type: multipart/form-data
// Query params: user_id (required)
func (s *Service) importCalendar(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.calendarService == nil {
		s.logger.Error("calendar service not configured")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "calendar service not available"})
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "user_id query parameter is required"})
		return
	}

	// Parse multipart form with a 10MB max size
	const maxFileSize = 10 << 20 // 10MB
	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		s.logger.Error("failed to parse multipart form", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid multipart form"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.logger.Error("failed to get file from form", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "file is required"})
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
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "unsupported file format. Supported formats: .ics, .csv",
		})
		return
	}

	if parseErr != nil {
		s.logger.Error("failed to parse file", "error", parseErr, "format", ext)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "failed to parse file",
		})
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

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// exportCalendar handles exporting calendar data to ICS or CSV format
// GET /api/v1/integration/export
// Query params: user_id (required), format (optional, default: ics), start_time, end_time
func (s *Service) exportCalendar(w http.ResponseWriter, r *http.Request) {
	// Check service availability
	if s.calendarService == nil {
		w.Header().Set("Content-Type", "application/json")
		s.logger.Error("calendar service not configured")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "calendar service not available"})
		return
	}

	// Parse and validate parameters
	userID, format, startTime, endTime, status, err := s.parseExportParams(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Fetch and filter events
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	events, err := s.fetchEventsFiltered(ctx, startTime, endTime, userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		s.logger.Error("failed to fetch events for export", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to fetch events"})
		return
	}

	// Generate export data
	data, contentType, filename, err := s.generateExportData(format, events)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		s.logger.Error("failed to generate export", "error", err, "format", format)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to generate export"})
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
	_, err = w.Write(data)
	if err != nil {
		s.logger.Error("failed to write export response", "error", err)
	}
}

// parseExportParams parses and validates export query parameters.
// Returns userID, format, startTime, endTime, httpStatus (non-0 when err!=nil), and error.
func (s *Service) parseExportParams(r *http.Request) (string, string, int64, int64, int, error) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		return "", "", 0, 0, http.StatusBadRequest, fmt.Errorf("user_id query parameter is required")
	}

	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "" {
		format = "ics"
	}
	if format != "ics" && format != "csv" {
		return "", "", 0, 0, http.StatusBadRequest, fmt.Errorf("unsupported format. Supported formats: ics, csv")
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

	return userID, format, startTime, endTime, 0, nil
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
// GET /api/v1/integration/google/auth?user_id=xxx
func (s *Service) googleAuth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if !s.googleService.IsConfigured() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Google Calendar integration is not configured",
		})
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "user_id query parameter is required",
		})
		return
	}

	authURL, err := s.googleService.GetAuthURL(userID)
	if err != nil {
		s.logger.Error("failed to generate auth URL", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "failed to generate authorization URL",
		})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{
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

	// Redirect to success page or return JSON
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Google Calendar connected successfully",
		"user_id": userID,
	})
}

// googleDisconnect disconnects Google Calendar for a user
// POST /api/v1/integration/google/disconnect
func (s *Service) googleDisconnect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		UserID string `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	if req.UserID == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "user_id is required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := s.googleService.Disconnect(ctx, req.UserID); err != nil {
		s.logger.Error("failed to disconnect Google Calendar", "error", err, "user_id", req.UserID)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to disconnect"})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Google Calendar disconnected successfully",
	})
}

// googleStatus returns the Google Calendar connection status for a user
// GET /api/v1/integration/google/status?user_id=xxx
func (s *Service) googleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if !s.googleService.IsConfigured() {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"configured": false,
			"connected":  false,
			"message":    "Google Calendar integration is not configured",
		})
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "user_id query parameter is required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	status, err := s.googleService.GetConnectionStatus(ctx, userID)
	if err != nil {
		s.logger.Error("failed to get connection status", "error", err, "user_id", userID)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to get status"})
		return
	}

	status["configured"] = true
	_ = json.NewEncoder(w).Encode(status)
}

// googleSync triggers a sync between local calendar and Google Calendar
// POST /api/v1/integration/google/sync
func (s *Service) googleSync(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		UserID    string `json:"user_id"`
		Direction string `json:"direction"`  // "from_google", "to_google", or "bidirectional"
		StartTime string `json:"start_time"` // optional, RFC3339 format
		EndTime   string `json:"end_time"`   // optional, RFC3339 format
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	if req.UserID == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "user_id is required"})
		return
	}

	// Default direction
	if req.Direction == "" {
		req.Direction = "from_google"
	}

	// Parse time range (default to 30 days from now)
	now := time.Now()
	startTime := now.AddDate(0, -1, 0) // 1 month ago
	endTime := now.AddDate(0, 1, 0)    // 1 month from now

	if req.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, req.StartTime); err == nil {
			startTime = t
		}
	}
	if req.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, req.EndTime); err == nil {
			endTime = t
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	result := map[string]interface{}{
		"direction": req.Direction,
	}

	switch req.Direction {
	case "from_google":
		count, err := s.googleService.SyncFromGoogle(ctx, req.UserID, startTime, endTime)
		if err != nil {
			s.logger.Error("sync from Google failed", "error", err, "user_id", req.UserID)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("sync failed: %v", err)})
			return
		}
		result["imported_count"] = count
		result["message"] = "Sync from Google Calendar completed"

	case "to_google":
		count, err := s.googleService.SyncToGoogle(ctx, req.UserID, startTime, endTime)
		if err != nil {
			s.logger.Error("sync to Google failed", "error", err, "user_id", req.UserID)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("sync failed: %v", err)})
			return
		}
		result["exported_count"] = count
		result["message"] = "Sync to Google Calendar completed"

	case "bidirectional":
		importCount, err := s.googleService.SyncFromGoogle(ctx, req.UserID, startTime, endTime)
		if err != nil {
			s.logger.Warn("sync from Google failed during bidirectional sync", "error", err)
		}
		exportCount, err := s.googleService.SyncToGoogle(ctx, req.UserID, startTime, endTime)
		if err != nil {
			s.logger.Warn("sync to Google failed during bidirectional sync", "error", err)
		}
		result["imported_count"] = importCount
		result["exported_count"] = exportCount
		result["message"] = "Bidirectional sync completed"

	default:
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid direction. Use 'from_google', 'to_google', or 'bidirectional'",
		})
		return
	}

	_ = json.NewEncoder(w).Encode(result)
}

// googleWebhook handles incoming push notifications from Google Calendar
// POST /api/v1/integration/google/webhook
func (s *Service) googleWebhook(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := s.googleService.HandleWebhook(ctx, r); err != nil {
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
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		UserID string `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	if req.UserID == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "user_id is required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	channel, err := s.googleService.SetupWatch(ctx, req.UserID)
	if err != nil {
		s.logger.Error("failed to setup watch", "error", err, "user_id", req.UserID)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("failed to setup watch: %v", err)})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message":     "Watch channel created successfully",
		"channel_id":  channel.Id,
		"resource_id": channel.ResourceId,
		"expiration":  channel.Expiration,
	})
}
