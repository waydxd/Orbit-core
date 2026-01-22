package location

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
	"github.com/waydxd/Orbit-core/pkg/middleware"
)

// Service represents the Location Service
type Service struct {
	config *config.Config
	logger *logger.Logger
	repo   Repository
}

// NewService creates a new Location Service
func NewService(cfg *config.Config, log *logger.Logger, repo Repository) *Service {
	return &Service{
		config: cfg,
		logger: log,
		repo:   repo,
	}
}

// RegisterRoutes registers location routes
func (s *Service) RegisterRoutes(router *mux.Router) {
	locationRouter := router.PathPrefix("/location").Subrouter()

	locationRouter.HandleFunc("/track", s.trackLocation).Methods("POST")
	locationRouter.HandleFunc("/history", s.getLocationHistory).Methods("GET")
	locationRouter.HandleFunc("/current", s.getCurrentLocation).Methods("GET")
	locationRouter.HandleFunc("/nearby", s.findNearby).Methods("GET")
}

// LocationRequest represents a location tracking request
type LocationRequest struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Address   string  `json:"address,omitempty"`
}

// trackLocation handles location tracking
func (s *Service) trackLocation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	var req LocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("failed to decode track location request", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"}); err != nil {
			s.logger.Error("failed to write track location error response", "error", err)
			return
		}
		return
	}

	location := &models.Location{
		ID:        uuid.New().String(),
		UserID:    userID,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
		Address:   req.Address,
		Timestamp: time.Now(),
		CreatedAt: time.Now(),
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := s.repo.CreateLocation(ctx, location); err != nil {
		s.logger.Error("failed to save location", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "failed to save location"}); err != nil {
			s.logger.Error("failed to write track location error response", "error", err)
			return
		}
		return
	}

	s.logger.Info("Location tracked",
		"user_id", userID,
		"lat", req.Latitude,
		"lng", req.Longitude,
	)

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(location); err != nil {
		s.logger.Error("failed to write track location success response", "error", err)
		return
	}
}

// getLocationHistory retrieves location history for a user
func (s *Service) getLocationHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	locations, err := s.repo.GetLocationHistory(ctx, userID, limit)
	if err != nil {
		s.logger.Error("failed to get location history", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "failed to get history"}); err != nil {
			s.logger.Error("failed to write getLocationHistory error response", "error", err)
			return
		}
		return
	}

	if locations == nil {
		locations = []*models.Location{}
	}

	if err := json.NewEncoder(w).Encode(locations); err != nil {
		s.logger.Error("failed to write getLocationHistory response", "error", err)
		return
	}
}

// getCurrentLocation retrieves current location for a user
func (s *Service) getCurrentLocation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	location, err := s.repo.GetCurrentLocation(ctx, userID)
	if err != nil {
		s.logger.Error("failed to get current location", "error", err)
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "no location found"}); err != nil {
			s.logger.Error("failed to write getCurrentLocation error response", "error", err)
			return
		}
		return
	}

	if err := json.NewEncoder(w).Encode(location); err != nil {
		s.logger.Error("failed to write getCurrentLocation response", "error", err)
		return
	}
}

// findNearby finds locations near given coordinates
func (s *Service) findNearby(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	latStr := r.URL.Query().Get("latitude")
	lngStr := r.URL.Query().Get("longitude")
	radiusStr := r.URL.Query().Get("radius")

	if latStr == "" || lngStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "latitude and longitude required"}); err != nil {
			s.logger.Error("failed to write findNearby error response", "error", err)
			return
		}
		return
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "invalid latitude"}); err != nil {
			s.logger.Error("failed to write findNearby error response", "error", err)
		}
		return
	}

	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "invalid longitude"}); err != nil {
			s.logger.Error("failed to write findNearby error response", "error", err)
		}
		return
	}

	radius := 10.0
	if radiusStr != "" {
		if r, err := strconv.ParseFloat(radiusStr, 64); err == nil && r > 0 {
			radius = r
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	locations, err := s.repo.FindNearby(ctx, lat, lng, radius)
	if err != nil {
		s.logger.Error("failed to find nearby locations", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "failed to find nearby locations"}); err != nil {
			s.logger.Error("failed to write findNearby error response", "error", err)
			return
		}
		return
	}

	if locations == nil {
		locations = []*models.Location{}
	}

	if err := json.NewEncoder(w).Encode(locations); err != nil {
		s.logger.Error("failed to write findNearby response", "error", err)
		return
	}
}
