package hashtag

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
	pb "github.com/waydxd/Orbit-core/pkg/hashtag/pb"
)

// Client handles communication with the hashtag service
type Client struct {
	config *config.HashtagConfig
	logger *logger.Logger
	conn   *grpc.ClientConn
	client pb.HashtagServiceClient
}

// NewClient creates a new hashtag client with retry logic
func NewClient(cfg *config.HashtagConfig, log *logger.Logger) (*Client, error) {
	if !cfg.Enabled {
		log.Info("Hashtag service is disabled")
		return &Client{config: cfg, logger: log}, nil
	}

	// Setup gRPC connection options
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()), // Use WithTransportSecurity for TLS in production
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                time.Duration(cfg.GRPC.KeepAlive) * time.Second,
			Timeout:             time.Duration(cfg.GRPC.KeepAliveTimeout) * time.Second,
			PermitWithoutStream: true,
		}),
	}

	// Connect to hashtag service
	addr := fmt.Sprintf("%s:%d", cfg.GRPC.Host, cfg.GRPC.Port)
	log.Info("Connecting to hashtag service", "address", addr)

	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		log.Warn("Failed to create gRPC client for hashtag service", "error", err)
		// Return a client with nil connection - service will gracefully degrade
		return &Client{config: cfg, logger: log}, nil
	}

	client := pb.NewHashtagServiceClient(conn)

	hc := &Client{
		config: cfg,
		logger: log,
		conn:   conn,
		client: client,
	}

	// Test connection with retries
	connected := false
	for attempt := 0; attempt < cfg.GRPC.MaxRetries; attempt++ {
		if err := hc.testConnection(); err == nil {
			log.Info("✓ Successfully connected to hashtag service", "attempt", attempt+1)
			connected = true
			break
		} else {
			if attempt < cfg.GRPC.MaxRetries-1 {
				backoff := time.Duration(100*(1<<uint(attempt))) * time.Millisecond
				log.Info("Hashtag service connection attempt failed, retrying",
					"attempt", attempt+1,
					"backoff_ms", backoff.Milliseconds(),
					"error", err)
				time.Sleep(backoff)
			}
		}
	}

	if !connected {
		log.Warn("Failed to connect to hashtag service after retries - service will run in degraded mode",
			"address", addr,
			"attempts", cfg.GRPC.MaxRetries)
		// Don't fail - continue with client that will gracefully degrade
	}

	return hc, nil
}

// testConnection tests the connection by calling health check
func (hc *Client) testConnection() error {
	if !hc.config.Enabled || hc.client == nil {
		return fmt.Errorf("client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(hc.config.GRPC.Timeout)*time.Second)
	defer cancel()

	health, err := hc.client.HealthCheck(ctx, &pb.HealthRequest{})
	if err != nil {
		return err
	}

	hc.logger.Info("Hashtag service health check",
		"model_version", health.ModelVersion,
		"num_hashtags", health.NumHashtags,
		"f1_score", health.F1Score,
		"device", health.Device)

	return nil
}

// Close closes the gRPC connection
func (hc *Client) Close() error {
	if hc.conn != nil {
		hc.logger.Info("Closing hashtag client connection")
		return hc.conn.Close()
	}
	return nil
}

// PredictHashtags predicts hashtags for the given event text with retry logic
func (hc *Client) PredictHashtags(ctx context.Context, eventText string, useBart bool, threshold float64) (*pb.PredictResponse, error) {
	if !hc.config.Enabled || hc.client == nil {
		return nil, fmt.Errorf("hashtag service not available")
	}

	if eventText == "" {
		return nil, fmt.Errorf("event text cannot be empty")
	}

	req := &pb.PredictRequest{
		EventText: eventText,
		UseBart:   &useBart,
		Threshold: &threshold,
	}

	var resp *pb.PredictResponse
	var err error

	// Retry with exponential backoff
	maxRetries := hc.config.GRPC.MaxRetries
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Create timeout context for each attempt
		attemptCtx, cancel := context.WithTimeout(ctx, time.Duration(hc.config.GRPC.Timeout)*time.Second)

		resp, err = hc.client.PredictHashtags(attemptCtx, req)
		cancel()

		if err == nil {
			hc.logger.Info("Predicted hashtags",
				"num_suggested", len(resp.Suggested),
				"event_text", truncate(eventText, 50),
				"used_bart", resp.UsedBart,
				"inference_time_ms", resp.InferenceTimeMs,
				"attempt", attempt+1)
			return resp, nil
		}

		// If this was the last attempt, return the error
		if attempt >= maxRetries {
			break
		}

		// Exponential backoff: 100ms, 200ms, 400ms, etc.
		backoffDuration := time.Duration(100*(1<<uint(attempt))) * time.Millisecond
		hc.logger.Warn("Prediction attempt failed, retrying",
			"attempt", attempt+1,
			"max_retries", maxRetries,
			"backoff_ms", backoffDuration.Milliseconds(),
			"error", err)

		// Wait before retrying
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoffDuration):
			// Continue to next retry
		}
	}

	return nil, fmt.Errorf("prediction failed after %d attempts: %w", maxRetries+1, err)
}

// CollectData collects user feedback for incremental learning
func (hc *Client) CollectData(ctx context.Context, userID int32, eventText string, selectedHashtags []string, timestamp string) error {
	if !hc.config.Enabled || hc.client == nil {
		return nil // Silently skip if disabled
	}

	if len(selectedHashtags) == 0 {
		return nil // Nothing to collect
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(hc.config.GRPC.Timeout)*time.Second)
	defer cancel()

	req := &pb.CollectDataRequest{
		UserId:           userID,
		EventText:        eventText,
		SelectedHashtags: selectedHashtags,
		Timestamp:        timestamp,
		Source:           "orbit-core",
	}

	resp, err := hc.client.CollectData(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to collect data: %w", err)
	}

	if resp.Status != "success" {
		return fmt.Errorf("collection failed: %s", resp.Message)
	}

	hc.logger.Info("Collected hashtag feedback",
		"user_id", userID,
		"num_hashtags", len(selectedHashtags),
		"samples_collected", resp.SamplesCollected)

	return nil
}

// HealthCheck checks if the hashtag service is healthy
func (hc *Client) HealthCheck(ctx context.Context) (*pb.HealthResponse, error) {
	if !hc.config.Enabled || hc.client == nil {
		return nil, fmt.Errorf("hashtag service not available")
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(hc.config.GRPC.Timeout)*time.Second)
	defer cancel()

	return hc.client.HealthCheck(ctx, &pb.HealthRequest{})
}

// IsAvailable checks if the service is available
func (hc *Client) IsAvailable() bool {
	if !hc.config.Enabled || hc.client == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	health, err := hc.client.HealthCheck(ctx, &pb.HealthRequest{})
	if err != nil {
		return false
	}

	return health.Status == "healthy"
}

// truncate truncates a string to the given length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

