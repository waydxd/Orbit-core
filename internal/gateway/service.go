package gateway

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
	"github.com/waydxd/Orbit-core/pkg/middleware"
)

// Service represents the Gateway Service
type Service struct {
	config         *config.Config
	logger         *logger.Logger
	router         *mux.Router
	rateLimiter    *middleware.RateLimiter
	authMiddleware *middleware.AuthMiddleware
	services       ServiceConfig
}

// ServiceConfig holds references to other services
type ServiceConfig struct {
	AuthService        AuthServiceInterface
	CalendarService    CalendarServiceInterface
	LocationService    LocationServiceInterface
	IntegrationService IntegrationServiceInterface
	ChatService        ChatServiceInterface
	HabitService       HabitServiceInterface
}

// AuthServiceInterface defines methods for auth service
type AuthServiceInterface interface {
	RegisterRoutes(router *mux.Router)
	RegisterProtectedRoutes(router *mux.Router)
}

// CalendarServiceInterface defines methods for calendar service
type CalendarServiceInterface interface {
	RegisterRoutes(router *mux.Router)
	ListEventsAdapter(ctx context.Context, startTime, endTime int64, status string) ([]interface{}, error)
	CreateEventAdapter(ctx context.Context, event interface{}) (interface{}, error)
	UpdateEventAdapter(ctx context.Context, id string, event interface{}) (interface{}, error)
	DeleteEventAdapter(ctx context.Context, id string) error
}

// LocationServiceInterface defines methods for location service
type LocationServiceInterface interface {
	RegisterRoutes(router *mux.Router)
}

// IntegrationServiceInterface defines methods for integration service
type IntegrationServiceInterface interface {
	RegisterRoutes(router *mux.Router)
}

// ChatServiceInterface defines methods for chat service
type ChatServiceInterface interface {
	RegisterRoutes(router *mux.Router)
}

// HabitServiceInterface defines methods for habit service
type HabitServiceInterface interface {
	RegisterRoutes(router *mux.Router)
}

// NewService creates a new Gateway Service
func NewService(cfg *config.Config, log *logger.Logger, svcConfig ServiceConfig) *Service {
	router := mux.NewRouter()

	// Initialize rate limiter with Redis
	rateLimiter := middleware.NewRateLimiter(
		cfg.Redis.RedisAddr(),
		cfg.Redis.Pass,
		cfg.Redis.DB,
		100,           // 100 requests
		1*time.Minute, // per minute
	)

	// Initialize auth middleware
	authMiddleware := middleware.NewAuthMiddleware(cfg.Auth.JWTKey)

	s := &Service{
		config:         cfg,
		logger:         log,
		router:         router,
		rateLimiter:    rateLimiter,
		authMiddleware: authMiddleware,
		services:       svcConfig,
	}

	s.setupRoutes()

	return s
}

// setupRoutes configures all routes
func (s *Service) setupRoutes() {
	// Health check endpoint
	s.router.HandleFunc("/health", s.healthCheck).Methods("GET")

	// API v1 routes
	apiRouter := s.router.PathPrefix("/api/v1").Subrouter()

	// Apply rate limiting middleware
	apiRouter.Use(s.rateLimiter.Middleware)

	// Public routes (Auth)
	s.services.AuthService.RegisterRoutes(apiRouter)

	// Protected routes
	protectedRouter := apiRouter.PathPrefix("").Subrouter()
	protectedRouter.Use(s.authMiddleware.Middleware)

	// Register service routes on protected router
	s.services.AuthService.RegisterProtectedRoutes(protectedRouter)
	s.services.CalendarService.RegisterRoutes(protectedRouter)
	s.services.LocationService.RegisterRoutes(protectedRouter)
	s.services.IntegrationService.RegisterRoutes(protectedRouter)
	s.services.ChatService.RegisterRoutes(protectedRouter)
	s.services.HabitService.RegisterRoutes(protectedRouter)
}

// Router returns the configured router
func (s *Service) Router() http.Handler {
	return s.router
}

// healthCheck is the health check endpoint handler
func (s *Service) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	s.logger.Info("Health check requested", "method", r.Method, "url", r.URL.String())

	_, err := w.Write([]byte(`{"status":"healthy","service":"gateway"}`))
	if err != nil {
		s.logger.Error("Error writing health check response", "err", err)
		return
	}
}
