package chat

import (
	"context"
	"time"

	"github.com/waydxd/Orbit-core/pkg/logger"
)

// CleanupJob handles background cleanup of expired actions 
type CleanupJob struct {
	service  *Service
	logger   *logger.Logger
	interval time.Duration
	stopChan chan struct{}
}

// NewCleanupJob creates a new cleanup job
func NewCleanupJob(service *Service, logger *logger.Logger, interval time.Duration) *CleanupJob {
	if interval == 0 {
		interval = 5 * time.Minute // Default to 5 minutes
	}

	return &CleanupJob{
		service:  service,
		logger:   logger,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

// Start begins the cleanup job
func (j *CleanupJob) Start(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	j.logger.Info("Cleanup job started", "interval", j.interval)

	// Run once immediately
	if err := j.service.CleanupExpiredActions(ctx); err != nil {
		j.logger.Error("Failed to run cleanup job", "error", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := j.service.CleanupExpiredActions(ctx); err != nil {
				j.logger.Error("Failed to run cleanup job", "error", err)
			}
		case <-j.stopChan:
			j.logger.Info("Cleanup job stopped")
			return
		case <-ctx.Done():
			j.logger.Info("Cleanup job stopped (context canceled)")
			return
		}
	}
}

// Stop stops the cleanup job
func (j *CleanupJob) Stop() {
	close(j.stopChan)
}
