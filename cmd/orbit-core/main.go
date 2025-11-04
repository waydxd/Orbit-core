package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/waydxd/Orbit-core/internal/agent"
	"github.com/waydxd/Orbit-core/internal/auth"
	"github.com/waydxd/Orbit-core/internal/calendar"
	"github.com/waydxd/Orbit-core/internal/gateway"
	"github.com/waydxd/Orbit-core/internal/integration"
	"github.com/waydxd/Orbit-core/internal/location"
	pb "github.com/waydxd/Orbit-Orbi/proto/calendar"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/grpc"
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
	// Initialize gRPC client for Orbi agent
	grpcClient, err := grpc.NewCalendarGRPCClient(cfg, log)
	if err != nil {
		log.Error("Failed to initialize gRPC client", "error", err)
		os.Exit(1)
	}
	defer grpcClient.Close()

	// Initialize agent service for AI interactions
	agentService := agent.NewService(cfg, log, grpcClient, calendarService)

	// Initialize gRPC server to expose CalendarDataService to Agent
	grpcServer, err := grpc.NewServer(grpc.ServerConfig{
		Port: cfg.GRPCServer.Port,
	}, log)
	if err != nil {
		log.Error("Failed to initialize gRPC server", "error", err)
		os.Exit(1)
	}

	// Register CalendarDataService with gRPC server
	pb.RegisterCalendarDataServiceServer(grpcServer, calendarService)

	// Start gRPC server in a goroutine
	go func() {
		if err := grpcServer.Start(); err != nil && err != grpc.ErrServerClosed {
			log.Error("gRPC server failed to start", "error", err)
			os.Exit(1)
		}
	}()
	defer grpcServer.Stop()

	// Initialize gateway (API Gateway/Router)
	gatewayService := gateway.NewService(cfg, log, gateway.ServiceConfig{
		AuthService:        authService,
		CalendarService:    calendarService,
		LocationService:    locationService,
		IntegrationService: integrationService,
		AgentService:       agentService,
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
	}

	log.Info("Server exited")
}
