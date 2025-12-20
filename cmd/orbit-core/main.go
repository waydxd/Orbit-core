package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/waydxd/Orbit-core/internal/agent"
	"github.com/waydxd/Orbit-core/internal/auth"
	"github.com/waydxd/Orbit-core/internal/calendar"
	"github.com/waydxd/Orbit-core/internal/chat"
	"github.com/waydxd/Orbit-core/internal/gateway"
	"github.com/waydxd/Orbit-core/internal/integration"
	"github.com/waydxd/Orbit-core/internal/location"
	"github.com/waydxd/Orbit-core/internal/shared/database"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/grpc"
	"github.com/waydxd/Orbit-core/pkg/logger"
	pb "github.com/waydxd/Orbit-core/proto/calendar"
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

	// Connect to database
	db, err := database.Connect(cfg.Database.ConnectionString())
	if err != nil {
		log.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer func(db *database.DB) {
		err := db.Close()
		if err != nil {
			log.Error("Failed to close database connection", "error", err)
		}
	}(db)

	defer database.DisconnectMongoDB()

	// Initialize MongoDB
	if err := database.InitMongoDB(cfg.MongoDB.URI); err != nil {
		log.Error("Failed to connect to MongoDB", "error", err)
		return
	}

	// Initialize repositories
	authRepo := auth.NewSQLRepository(db)
	eventRepo := calendar.NewSQLEventRepository(db)
	taskRepo := calendar.NewSQLTaskRepository(db)
	locationRepo := location.NewSQLRepository(db)
	chatRepo, err := chat.NewMongoRepository(context.Background(), database.MongoClient, cfg.Database.DBName)
	if err != nil {
		log.Error("Failed to initialize chat repository", "error", err)
		return
	}

	// Initialize services with repositories
	authService := auth.NewService(cfg, log, authRepo)
	calendarService := calendar.NewService(cfg, log, eventRepo, taskRepo)
	locationService := location.NewService(cfg, log, locationRepo)
	integrationService := integration.NewService(cfg, log)

	// Initialize gRPC client for Orbi agent
	grpcClient, err := grpc.NewCalendarGRPCClient(cfg, log)
	if err != nil {
		log.Error("Failed to initialize gRPC client", "error", err)
		return
	}
	defer func(grpcClient *grpc.CalendarGRPCClient) {
		err := grpcClient.Close()
		if err != nil {
			log.Error("Failed to close gRPC client", "error", err)
		}
	}(grpcClient)

	// Initialize agent service for AI interactions
	agentService := agent.NewService(cfg, log, grpcClient, calendarService)

	// Initialize chat service for chatbot functionality
	chatService := chat.NewService(cfg, log, chatRepo, grpcClient)

	// Start cleanup job for expired actions
	cleanupJob := chat.NewCleanupJob(chatService, log, 5*time.Minute)
	cancelContext, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	go cleanupJob.Start(cancelContext)

	// Initialize gRPC server to expose CalendarDataService to Agent
	grpcServer, err := grpc.NewServer(grpc.ServerConfig{
		Port: cfg.GRPCServer.Port,
	}, log)
	if err != nil {
		log.Error("Failed to initialize gRPC server", "error", err)
		return
	}

	// Register CalendarDataService with gRPC server
	pb.RegisterCalendarDataServiceServer(grpcServer.Underlying(), calendarService)

	// Start gRPC server in a goroutine
	go func() {
		if err := grpcServer.Start(); err != nil {
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
		ChatService:        chatService,
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
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
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
