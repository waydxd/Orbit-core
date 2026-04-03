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
	_ "time/tzdata"

	"github.com/hibiken/asynq"
	"github.com/waydxd/Orbit-core/internal/asset"
	"github.com/waydxd/Orbit-core/internal/auth"
	"github.com/waydxd/Orbit-core/internal/calendar"
	"github.com/waydxd/Orbit-core/internal/chat"
	"github.com/waydxd/Orbit-core/internal/gateway"
	"github.com/waydxd/Orbit-core/internal/habit"
	"github.com/waydxd/Orbit-core/internal/integration"
	"github.com/waydxd/Orbit-core/internal/location"
	"github.com/waydxd/Orbit-core/internal/notification"
	"github.com/waydxd/Orbit-core/internal/shared/database"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/fcm"
	"github.com/waydxd/Orbit-core/pkg/grpc"
	"github.com/waydxd/Orbit-core/pkg/logger"
	pb "github.com/waydxd/Orbit-core/proto/calendar"
)

func main() {
	// Initialize logger
	log := logger.New()
	log.Info("Starting Orbit-core monolith...")

	cfg, db, mongoCtx, mongoCancel := initDependencies(log)
	if cfg == nil {
		return
	}
	defer mongoCancel()

	authRepo, eventRepo, taskRepo, locationRepo, habitRepo := initSQLRepositories(db)
	chatRepo, assetRepo := initMongoRepositories(log, mongoCtx, cfg.MongoDB.DBName)
	if chatRepo == nil || assetRepo == nil {
		return
	}

	grpcClient, fcmClient, asynqClient, asynqInspector := initExternalClients(log, cfg)
	if grpcClient == nil {
		return
	}
	defer closeGRPCClient(grpcClient, log)
	defer closeAsynqClient(asynqClient, log)
	defer closeAsynqInspector(asynqInspector, log)

	habitService := habit.NewService(cfg, log, habitRepo)
	log.Info("Habit tracking service initialized successfully")
	assetService := asset.NewService(cfg, log, assetRepo, db)

	authService := auth.NewService(cfg, log, authRepo)
	calendarService := calendar.NewService(cfg, log, eventRepo, taskRepo, habitService)
	chatService := chat.NewService(cfg, log, chatRepo, grpcClient, calendarService)
	locationService := location.NewService(cfg, log, locationRepo)
	integrationService := integration.NewService(cfg, log)
	integrationService.SetCalendarService(calendarService)

	notificationRepo := notification.NewSQLRepository(db)
	notificationService := notification.NewService(cfg, log, notificationRepo, fcmClient, asynqClient, asynqInspector)
	asynqServer := initAsynqServer(cfg.Redis, log)
	notificationWorker := notification.NewWorker(notificationRepo, fcmClient, log, asynqServer)
	notificationWorker.Start()
	defer notificationWorker.Stop()

	actionInterceptor := grpc.NewActionInterceptor(log, chatRepo)
	grpcServer := initGRPCServer(log, cfg, actionInterceptor, calendarService)
	if grpcServer == nil {
		return
	}

	gatewayService := gateway.NewService(cfg, log, gateway.ServiceConfig{
		AuthService:         authService,
		CalendarService:     calendarService,
		LocationService:     locationService,
		IntegrationService:  integrationService,
		ChatService:         chatService,
		HabitService:        habitService,
		NotificationService: notificationService,
		AssetService:        assetService,
	})

	startHTTPServer(log, cfg, gatewayService)
}

func initDependencies(log *logger.Logger) (*config.Config, *database.DB, context.Context, context.CancelFunc) {
	cfg, err := config.Load()
	if err != nil {
		log.Error("Failed to load configuration", "error", err)
		return nil, nil, nil, nil
	}

	db, err := database.Connect(cfg.Database.ConnectionString())
	if err != nil {
		log.Error("Failed to connect to PostgreSQL", "error", err)
		return nil, nil, nil, nil
	}
	defer func(db *database.DB) {
		db.Close()
		log.Info("Closed PostgreSQL connection")
	}(db)

	defer database.DisconnectMongoDB()

	mongoURI := database.BuildMongoURI(cfg.MongoDB.User, cfg.MongoDB.Pass, cfg.MongoDB.Host, cfg.MongoDB.DBName)
	if err := database.InitMongoDB(mongoURI); err != nil {
		log.Error("Failed to connect to MongoDB", "error", err)
		return nil, nil, nil, nil
	}

	mongoCtx, mongoCancel := context.WithTimeout(context.Background(), 30*time.Second)
	return cfg, db, mongoCtx, mongoCancel
}

func initSQLRepositories(db *database.DB) (auth.Repository, calendar.EventRepository, calendar.TaskRepository, location.Repository, habit.Repository) {
	authRepo := auth.NewSQLRepository(db)
	eventRepo := calendar.NewSQLEventRepository(db)
	taskRepo := calendar.NewSQLTaskRepository(db)
	locationRepo := location.NewSQLRepository(db)
	habitRepo := habit.NewSQLRepository(db)
	return authRepo, eventRepo, taskRepo, locationRepo, habitRepo
}

func initMongoRepositories(log *logger.Logger, mongoCtx context.Context, dbName string) (chat.Repository, asset.Repository) {
	chatRepo, err := chat.NewMongoRepository(mongoCtx, database.MongoClient, dbName)
	if err != nil {
		log.Error("Failed to initialize chat repository", "error", err)
		return nil, nil
	}
	assetRepo, err := asset.NewMongoRepository(mongoCtx, database.MongoClient, dbName)
	if err != nil {
		log.Error("Failed to initialize asset repository", "error", err)
		return nil, nil
	}
	return chatRepo, assetRepo
}

func initExternalClients(log *logger.Logger, cfg *config.Config) (*grpc.CalendarGRPCClient, *fcm.Client, *asynq.Client, *asynq.Inspector) {
	grpcClient, err := grpc.NewCalendarGRPCClient(cfg, log)
	if err != nil {
		log.Error("Failed to initialize gRPC client", "error", err)
		return nil, nil, nil, nil
	}

	fcmClient, fcmErr := fcm.Init(context.Background(), cfg.Firebase.CredentialsJSON)
	if fcmErr != nil {
		log.Error("FCM client initialization failed; push notifications will be disabled", "error", fcmErr)
	}

	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.Redis.RedisAddr(),
		Password: cfg.Redis.Pass,
		DB:       cfg.Redis.DB,
	}
	asynqClient := asynq.NewClient(redisOpt)
	asynqInspector := asynq.NewInspector(redisOpt)

	return grpcClient, fcmClient, asynqClient, asynqInspector
}

func closeGRPCClient(grpcClient *grpc.CalendarGRPCClient, log *logger.Logger) {
	if err := grpcClient.Close(); err != nil {
		log.Error("Failed to close gRPC client", "error", err)
	}
}

func closeAsynqClient(asynqClient *asynq.Client, log *logger.Logger) {
	if err := asynqClient.Close(); err != nil {
		log.Error("Failed to close Asynq client", "error", err)
	}
}

func closeAsynqInspector(asynqInspector *asynq.Inspector, log *logger.Logger) {
	if err := asynqInspector.Close(); err != nil {
		log.Error("Failed to close Asynq inspector", "error", err)
	}
}

func initAsynqServer(cfg config.RedisConfig, log *logger.Logger) *asynq.Server {
	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr(),
		Password: cfg.Pass,
		DB:       cfg.DB,
	}
	return asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 10,
		Queues: map[string]int{
			notification.QueueDefault:  10,
			notification.QueueCritical: 50,
		},
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			log.Error("Asynq task failed", "type", task.Type(), "error", err)
		}),
	})
}

func initGRPCServer(log *logger.Logger, cfg *config.Config, actionInterceptor *grpc.ActionInterceptor, calendarService *calendar.Service) *grpc.Server {
	grpcServer, err := grpc.NewServer(grpc.ServerConfig{
		Port:         cfg.GRPCServer.Port,
		Interceptors: []grpc.UnaryServerInterceptor{actionInterceptor.UnaryInterceptor()},
	}, log)
	if err != nil {
		log.Error("Failed to initialize gRPC server", "error", err)
		return nil
	}

	pb.RegisterCalendarDataServiceServer(grpcServer.Underlying(), calendarService)
	pb.RegisterCalendarServiceServer(grpcServer.Underlying(), calendarService)

	go func() {
		if err := grpcServer.Start(); err != nil {
			log.Error("gRPC server failed to start", "error", err)
			os.Exit(1)
		}
	}()
	defer grpcServer.Stop()

	return grpcServer
}

func startHTTPServer(log *logger.Logger, cfg *config.Config, gatewayService *gateway.Service) {
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      gatewayService.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("Server starting", "port", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", "error", err)
	}

	log.Info("Server exited")
}
