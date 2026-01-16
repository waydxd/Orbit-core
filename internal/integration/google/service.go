package google

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gcal "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// CalendarServiceInterface defines the methods needed from calendar service
type CalendarServiceInterface interface {
	ListEventsAdapter(ctx context.Context, startTime, endTime int64, status string) ([]interface{}, error)
	CreateEventAdapter(ctx context.Context, event interface{}) (interface{}, error)
	UpdateEventAdapter(ctx context.Context, id string, event interface{}) (interface{}, error)
	DeleteEventAdapter(ctx context.Context, id string) error
}

// TokenStore provides storage for OAuth tokens
// In production, this should be backed by a database
type TokenStore interface {
	GetToken(ctx context.Context, userID string) (*oauth2.Token, error)
	SaveToken(ctx context.Context, userID string, token *oauth2.Token) error
	DeleteToken(ctx context.Context, userID string) error
}

// InMemoryTokenStore is a simple in-memory token store for development
// TODO: Replace with database-backed storage in production
type InMemoryTokenStore struct {
	mu     sync.RWMutex
	tokens map[string]*oauth2.Token
}

// NewInMemoryTokenStore creates a new in-memory token store
func NewInMemoryTokenStore() *InMemoryTokenStore {
	return &InMemoryTokenStore{
		tokens: make(map[string]*oauth2.Token),
	}
}

// GetToken retrieves a token for a user
func (s *InMemoryTokenStore) GetToken(_ context.Context, userID string) (*oauth2.Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	token, ok := s.tokens[userID]
	if !ok {
		return nil, errors.New("token not found")
	}
	return token, nil
}

// SaveToken saves a token for a user
func (s *InMemoryTokenStore) SaveToken(_ context.Context, userID string, token *oauth2.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[userID] = token
	return nil
}

// DeleteToken deletes a token for a user
func (s *InMemoryTokenStore) DeleteToken(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, userID)
	return nil
}

// pendingAuthEntry stores a userID and creation time for OAuth state validation
type pendingAuthEntry struct {
	userID    string
	createdAt time.Time
}

// Service provides Google Calendar integration
type Service struct {
	config          *config.Config
	logger          *logger.Logger
	oauthConfig     *oauth2.Config
	tokenStore      TokenStore
	calendarService CalendarServiceInterface

	// pendingAuth stores state -> pendingAuthEntry mapping for OAuth flow
	// Entries expire after pendingAuthTTL to prevent memory leaks
	pendingAuth   map[string]pendingAuthEntry
	pendingAuthMu sync.RWMutex
}

const (
	// pendingAuthTTL is the maximum time a pending auth state is valid
	pendingAuthTTL = 10 * time.Minute
	// pendingAuthMaxSize is the maximum number of pending auth entries before cleanup
	pendingAuthMaxSize = 1000
)

// NewService creates a new Google Calendar integration service
func NewService(cfg *config.Config, log *logger.Logger, tokenStore TokenStore) *Service {
	// Create OAuth2 config for Google Calendar
	// Using only CalendarEventsScope which provides both read and write access
	oauthConfig := &oauth2.Config{
		ClientID:     cfg.GoogleCalendar.ClientID,
		ClientSecret: cfg.GoogleCalendar.ClientSecret,
		RedirectURL:  cfg.GoogleCalendar.RedirectURL,
		Scopes: []string{
			gcal.CalendarEventsScope,
		},
		Endpoint: google.Endpoint,
	}

	return &Service{
		config:      cfg,
		logger:      log,
		oauthConfig: oauthConfig,
		tokenStore:  tokenStore,
		pendingAuth: make(map[string]pendingAuthEntry),
	}
}

// SetCalendarService sets the calendar service for sync operations
func (s *Service) SetCalendarService(calSvc CalendarServiceInterface) {
	s.calendarService = calSvc
}

// IsConfigured returns true if Google Calendar integration is configured
func (s *Service) IsConfigured() bool {
	return s.config.GoogleCalendar.ClientID != "" &&
		s.config.GoogleCalendar.ClientSecret != ""
}

// GetAuthURL generates the OAuth authorization URL for a user
func (s *Service) GetAuthURL(userID string) (string, error) {
	if !s.IsConfigured() {
		return "", errors.New("google calendar integration not configured")
	}

	// Generate a state token for CSRF protection
	state := uuid.New().String()

	// Store the state -> userID mapping with timestamp
	s.pendingAuthMu.Lock()
	// Clean up expired entries if map is getting large
	if len(s.pendingAuth) >= pendingAuthMaxSize {
		s.cleanupExpiredPendingAuth()
	}
	s.pendingAuth[state] = pendingAuthEntry{
		userID:    userID,
		createdAt: time.Now(),
	}
	s.pendingAuthMu.Unlock()

	// Generate the OAuth URL
	url := s.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	return url, nil
}

// cleanupExpiredPendingAuth removes expired entries from pendingAuth map
// Must be called with pendingAuthMu lock held
func (s *Service) cleanupExpiredPendingAuth() {
	now := time.Now()
	for state, entry := range s.pendingAuth {
		if now.Sub(entry.createdAt) > pendingAuthTTL {
			delete(s.pendingAuth, state)
		}
	}
}

// HandleCallback processes the OAuth callback from Google
func (s *Service) HandleCallback(ctx context.Context, state, code string) (string, error) {
	// Validate state and get userID
	s.pendingAuthMu.Lock()
	entry, ok := s.pendingAuth[state]
	if ok {
		delete(s.pendingAuth, state)
	}
	s.pendingAuthMu.Unlock()

	if !ok {
		return "", errors.New("invalid state parameter")
	}

	// Check if the state has expired
	if time.Since(entry.createdAt) > pendingAuthTTL {
		return "", errors.New("state parameter has expired")
	}

	userID := entry.userID

	// Exchange the authorization code for tokens
	token, err := s.oauthConfig.Exchange(ctx, code)
	if err != nil {
		s.logger.Error("failed to exchange code for token", "error", err)
		return "", fmt.Errorf("failed to exchange code: %w", err)
	}

	// Save the token
	if err := s.tokenStore.SaveToken(ctx, userID, token); err != nil {
		s.logger.Error("failed to save token", "error", err, "user_id", userID)
		return "", fmt.Errorf("failed to save token: %w", err)
	}

	s.logger.Info("Google Calendar connected successfully", "user_id", userID)
	return userID, nil
}

// Disconnect removes the Google Calendar connection for a user
func (s *Service) Disconnect(ctx context.Context, userID string) error {
	if err := s.tokenStore.DeleteToken(ctx, userID); err != nil {
		s.logger.Error("failed to delete token", "error", err, "user_id", userID)
		return fmt.Errorf("failed to disconnect: %w", err)
	}
	s.logger.Info("Google Calendar disconnected", "user_id", userID)
	return nil
}

// getClient returns an authenticated HTTP client for a user
func (s *Service) getClient(ctx context.Context, userID string) (*http.Client, error) {
	token, err := s.tokenStore.GetToken(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not connected to Google Calendar: %w", err)
	}

	// Create a token source that will auto-refresh
	tokenSource := s.oauthConfig.TokenSource(ctx, token)

	// Get a potentially refreshed token
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	// Save the refreshed token if it changed
	if newToken.AccessToken != token.AccessToken {
		if err := s.tokenStore.SaveToken(ctx, userID, newToken); err != nil {
			s.logger.Warn("failed to save refreshed token", "error", err, "user_id", userID)
		}
	}

	return oauth2.NewClient(ctx, tokenSource), nil
}

// getCalendarService creates a Google Calendar service for a user
func (s *Service) getCalendarService(ctx context.Context, userID string) (*gcal.Service, error) {
	client, err := s.getClient(ctx, userID)
	if err != nil {
		return nil, err
	}

	calService, err := gcal.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create calendar service: %w", err)
	}

	return calService, nil
}

// ListGoogleEvents fetches events from Google Calendar
func (s *Service) ListGoogleEvents(ctx context.Context, userID string, startTime, endTime time.Time) ([]*gcal.Event, error) {
	calService, err := s.getCalendarService(ctx, userID)
	if err != nil {
		return nil, err
	}

	events, err := calService.Events.List("primary").
		TimeMin(startTime.Format(time.RFC3339)).
		TimeMax(endTime.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		Do()
	if err != nil {
		s.logger.Error("failed to list Google Calendar events", "error", err, "user_id", userID)
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	return events.Items, nil
}

// SyncFromGoogle imports events from Google Calendar to our system
func (s *Service) SyncFromGoogle(ctx context.Context, userID string, startTime, endTime time.Time) (int, error) {
	if s.calendarService == nil {
		return 0, errors.New("calendar service not configured")
	}

	googleEvents, err := s.ListGoogleEvents(ctx, userID, startTime, endTime)
	if err != nil {
		return 0, err
	}

	importedCount := 0
	for _, gEvent := range googleEvents {
		event := googleEventToModel(gEvent, userID)
		if event == nil {
			continue
		}

		_, err := s.calendarService.CreateEventAdapter(ctx, event)
		if err != nil {
			s.logger.Warn("failed to import Google event",
				"event_id", gEvent.Id,
				"title", gEvent.Summary,
				"error", err)
			continue
		}
		importedCount++
	}

	s.logger.Info("Sync from Google completed",
		"user_id", userID,
		"imported", importedCount,
		"total", len(googleEvents))

	return importedCount, nil
}

// SyncToGoogle exports events from our system to Google Calendar
func (s *Service) SyncToGoogle(ctx context.Context, userID string, startTime, endTime time.Time) (int, error) {
	if s.calendarService == nil {
		return 0, errors.New("calendar service not configured")
	}

	calService, err := s.getCalendarService(ctx, userID)
	if err != nil {
		return 0, err
	}

	// Get our events
	eventsInterface, err := s.calendarService.ListEventsAdapter(ctx, startTime.Unix(), endTime.Unix(), "")
	if err != nil {
		return 0, fmt.Errorf("failed to list local events: %w", err)
	}

	exportedCount := 0
	for _, e := range eventsInterface {
		event, ok := e.(*models.Event)
		if !ok || event.UserID != userID {
			continue
		}

		gEvent := modelToGoogleEvent(event)
		_, err := calService.Events.Insert("primary", gEvent).Do()
		if err != nil {
			s.logger.Warn("failed to export event to Google",
				"event_id", event.ID,
				"title", event.Title,
				"error", err)
			continue
		}
		exportedCount++
	}

	s.logger.Info("Sync to Google completed",
		"user_id", userID,
		"exported", exportedCount)

	return exportedCount, nil
}

// googleEventToModel converts a Google Calendar event to our model
func googleEventToModel(gEvent *gcal.Event, userID string) *models.Event {
	if gEvent == nil || gEvent.Summary == "" {
		return nil
	}

	event := &models.Event{
		ID:          uuid.New().String(),
		UserID:      userID,
		Title:       gEvent.Summary,
		Description: gEvent.Description,
		Location:    gEvent.Location,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Parse start time
	if gEvent.Start != nil {
		if gEvent.Start.DateTime != "" {
			if t, err := time.Parse(time.RFC3339, gEvent.Start.DateTime); err == nil {
				event.StartTime = t
			}
		} else if gEvent.Start.Date != "" {
			if t, err := time.Parse("2006-01-02", gEvent.Start.Date); err == nil {
				event.StartTime = t
			}
		}
	}

	// Parse end time
	if gEvent.End != nil {
		if gEvent.End.DateTime != "" {
			if t, err := time.Parse(time.RFC3339, gEvent.End.DateTime); err == nil {
				event.EndTime = t
			}
		} else if gEvent.End.Date != "" {
			if t, err := time.Parse("2006-01-02", gEvent.End.Date); err == nil {
				event.EndTime = t
			}
		}
	}

	return event
}

// modelToGoogleEvent converts our event model to a Google Calendar event
func modelToGoogleEvent(event *models.Event) *gcal.Event {
	return &gcal.Event{
		Summary:     event.Title,
		Description: event.Description,
		Location:    event.Location,
		Start: &gcal.EventDateTime{
			DateTime: event.StartTime.Format(time.RFC3339),
		},
		End: &gcal.EventDateTime{
			DateTime: event.EndTime.Format(time.RFC3339),
		},
	}
}

// WatchNotification represents a Google Calendar push notification
type WatchNotification struct {
	ChannelID         string `json:"channel_id"`
	ResourceID        string `json:"resource_id"`
	ResourceURI       string `json:"resource_uri"`
	ResourceState     string `json:"resource_state"`
	ChannelExpiration string `json:"channel_expiration"`
}

// HandleWebhook processes incoming Google Calendar push notifications
func (s *Service) HandleWebhook(ctx context.Context, r *http.Request) error {
	// Parse the notification headers
	channelID := r.Header.Get("X-Goog-Channel-ID")
	resourceState := r.Header.Get("X-Goog-Resource-State")
	resourceID := r.Header.Get("X-Goog-Resource-ID")

	s.logger.Info("Google Calendar webhook received",
		"channel_id", channelID,
		"resource_state", resourceState,
		"resource_id", resourceID)

	// Handle different resource states
	switch resourceState {
	case "sync":
		// Initial sync notification - no action needed
		s.logger.Debug("Sync notification received, ignoring")
	case "exists":
		// Resource changed - trigger a sync
		// In a real implementation, you would look up the userID from the channelID
		s.logger.Info("Resource change detected, triggering sync",
			"resource_id", resourceID)
		// TODO: Implement incremental sync based on channel mapping
	case "not_exists":
		// Resource deleted
		s.logger.Info("Resource deletion detected",
			"resource_id", resourceID)
	}

	return nil
}

// SetupWatch creates a watch channel for push notifications
func (s *Service) SetupWatch(ctx context.Context, userID string) (*gcal.Channel, error) {
	if s.config.GoogleCalendar.WebhookURL == "" {
		return nil, errors.New("webhook URL not configured")
	}

	calService, err := s.getCalendarService(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Create a unique channel ID
	channelID := uuid.New().String()

	channel := &gcal.Channel{
		Id:      channelID,
		Type:    "web_hook",
		Address: s.config.GoogleCalendar.WebhookURL,
	}

	watchResponse, err := calService.Events.Watch("primary", channel).Do()
	if err != nil {
		s.logger.Error("failed to setup watch channel", "error", err, "user_id", userID)
		return nil, fmt.Errorf("failed to setup watch: %w", err)
	}

	s.logger.Info("Watch channel created",
		"user_id", userID,
		"channel_id", watchResponse.Id,
		"expiration", watchResponse.Expiration)

	return watchResponse, nil
}

// StopWatch stops a watch channel
func (s *Service) StopWatch(ctx context.Context, userID string, channelID, resourceID string) error {
	calService, err := s.getCalendarService(ctx, userID)
	if err != nil {
		return err
	}

	channel := &gcal.Channel{
		Id:         channelID,
		ResourceId: resourceID,
	}

	err = calService.Channels.Stop(channel).Do()
	if err != nil {
		s.logger.Error("failed to stop watch channel", "error", err, "channel_id", channelID)
		return fmt.Errorf("failed to stop watch: %w", err)
	}

	s.logger.Info("Watch channel stopped", "channel_id", channelID)
	return nil
}

// GetConnectionStatus returns the connection status for a user
func (s *Service) GetConnectionStatus(ctx context.Context, userID string) (map[string]interface{}, error) {
	token, err := s.tokenStore.GetToken(ctx, userID)
	if err != nil {
		return map[string]interface{}{
			"connected": false,
			"message":   "Not connected to Google Calendar",
		}, nil
	}

	status := map[string]interface{}{
		"connected":    true,
		"token_expiry": token.Expiry.Format(time.RFC3339),
	}

	// Check if we can access the calendar
	calService, err := s.getCalendarService(ctx, userID)
	if err != nil {
		status["status"] = "error"
		status["message"] = "Failed to connect to Google Calendar"
		return status, nil
	}

	// Try to get calendar list to verify connection
	_, err = calService.CalendarList.Get("primary").Do()
	if err != nil {
		status["status"] = "error"
		status["message"] = "Connection expired or invalid"
		return status, nil
	}

	status["status"] = "active"
	status["message"] = "Connected to Google Calendar"
	return status, nil
}
