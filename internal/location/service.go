package location

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
)

// Service represents the Location Service
type Service struct {
	config *config.Config
	logger *logger.Logger
}

// NewService creates a new Location Service
func NewService(cfg *config.Config, log *logger.Logger) *Service {
	return &Service{
		config: cfg,
		logger: log,
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
	UserID    string  `json:"user_id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Address   string  `json:"address,omitempty"`
}

// trackLocation handles location tracking
func (s *Service) trackLocation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req LocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	// TODO: Store location in PostgreSQL
	s.logger.Info("Location tracked",
		"user_id", req.UserID,
		"lat", req.Latitude,
		"lng", req.Longitude,
	)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "location tracked successfully"})
}

// getLocationHistory retrieves location history for a user
func (s *Service) getLocationHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "user_id required"})
		return
	}

	// TODO: Fetch location history from PostgreSQL
	locations := []models.Location{}

	json.NewEncoder(w).Encode(locations)
}

// getCurrentLocation retrieves current location for a user
func (s *Service) getCurrentLocation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "user_id required"})
		return
	}

	// TODO: Fetch current location from PostgreSQL
	s.logger.Info("Fetching current location", "user_id", userID)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id":   userID,
		"latitude":  0.0,
		"longitude": 0.0,
	})
}

// findNearby finds nearby locations/places
func (s *Service) findNearby(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	lat := r.URL.Query().Get("lat")
	lng := r.URL.Query().Get("lng")

	if lat == "" || lng == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "lat and lng required"})
		return
	}

	// TODO: Implement geolocation functionality
	s.logger.Info("Finding nearby locations", "lat", lat, "lng", lng)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode([]interface{}{})
}
