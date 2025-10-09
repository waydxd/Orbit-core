package integration

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
)

// Service represents the Integration Service
type Service struct {
	config *config.Config
	logger *logger.Logger
}

// NewService creates a new Integration Service
func NewService(cfg *config.Config, log *logger.Logger) *Service {
	return &Service{
		config: cfg,
		logger: log,
	}
}

// RegisterRoutes registers integration routes
func (s *Service) RegisterRoutes(router *mux.Router) {
	integrationRouter := router.PathPrefix("/integration").Subrouter()

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
