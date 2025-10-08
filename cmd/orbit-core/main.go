package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/waydxd/Orbit-core/internal/auth"
	"github.com/waydxd/Orbit-core/internal/calendar"
	"github.com/waydxd/Orbit-core/internal/gateway"
	"github.com/waydxd/Orbit-core/internal/integration"
	"github.com/waydxd/Orbit-core/internal/location"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
)

func main() {
	// Initialize logger
	log := logger.New()
	log.Info("Starting Orbit-core monolith...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Initialize services
	authService := auth.NewService(cfg, log)
	calendarService := calendar.NewService(cfg, log)
	locationService := location.NewService(cfg, log)
	integrationService := integration.NewService(cfg, log)

	// Initialize gateway (API Gateway/Router)
	gatewayService := gateway.NewService(cfg, log, gateway.ServiceConfig{
		AuthService:        authService,
		CalendarService:    calendarService,
		LocationService:    locationService,
		IntegrationService: integrationService,
	})

	// Start HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      gatewayService.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info("Server starting", "port", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	log.Info("Server exited")
}
