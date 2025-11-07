package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
	pb "github.com/waydxd/Orbit-core/proto/calendar"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

// CalendarGRPCClient wraps the gRPC client for calendar service
type CalendarGRPCClient struct {
	conn   *grpc.ClientConn
	client pb.CalendarServiceClient
	logger *logger.Logger
}

// NewCalendarGRPCClient creates a new gRPC client for the calendar service
func NewCalendarGRPCClient(cfg *config.Config, log *logger.Logger) (*CalendarGRPCClient, error) {
	// Connect to Orbi agent gRPC server
	addr := fmt.Sprintf("%s:%d", cfg.Orbi.Host, cfg.Orbi.Port)

	conn, err := grpc.NewClient(
		addr,
		// Consider using grpc.WithTransportCredentials() with TLS credentials for production.
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Error("Failed to connect to Orbi agent", "address", addr, "error", err)
		return nil, err
	}

	client := pb.NewCalendarServiceClient(conn)

	return &CalendarGRPCClient{
		conn:   conn,
		client: client,
		logger: log,
	}, nil
}

// Close closes the gRPC connection
func (c *CalendarGRPCClient) Close() error {
	return c.conn.Close()
}

// GetConnection returns the underlying gRPC connection
func (c *CalendarGRPCClient) GetConnection() *grpc.ClientConn {
	return c.conn
}

// GetCalendarServiceClient returns the gRPC CalendarService client
func (c *CalendarGRPCClient) GetCalendarServiceClient() pb.CalendarServiceClient {
	return c.client
}

// CreateEvent calls Agent's CreateEvent RPC
func (c *CalendarGRPCClient) CreateEvent(ctx context.Context, req *pb.CreateEventRequest) (*pb.CreateEventResponse, error) {
	c.logger.Info("Calling Agent's CreateEvent", "title", req.Title)
	return c.client.CreateEvent(ctx, req)
}

// GetEvents calls Agent's GetEvents RPC
func (c *CalendarGRPCClient) GetEvents(ctx context.Context, req *pb.GetEventsRequest) (*pb.GetEventsResponse, error) {
	c.logger.Info("Calling Agent's GetEvents", "start_time", req.StartTime, "end_time", req.EndTime)
	return c.client.GetEvents(ctx, req)
}

// UpdateEvent calls Agent's UpdateEvent RPC
func (c *CalendarGRPCClient) UpdateEvent(ctx context.Context, req *pb.UpdateEventRequest) (*pb.UpdateEventResponse, error) {
	c.logger.Info("Calling Agent's UpdateEvent", "id", req.Id)
	return c.client.UpdateEvent(ctx, req)
}

// DeleteEvent calls Agent's DeleteEvent RPC
func (c *CalendarGRPCClient) DeleteEvent(ctx context.Context, req *pb.DeleteEventRequest) (*pb.DeleteEventResponse, error) {
	c.logger.Info("Calling Agent's DeleteEvent", "id", req.Id)
	return c.client.DeleteEvent(ctx, req)
}

// GetAvailableSlots calls Agent's GetAvailableSlots RPC
func (c *CalendarGRPCClient) GetAvailableSlots(ctx context.Context, req *pb.GetAvailableSlotsRequest) (*pb.GetAvailableSlotsResponse, error) {
	c.logger.Info("Calling Agent's GetAvailableSlots", "start_time", req.StartTime, "end_time", req.EndTime)
	return c.client.GetAvailableSlots(ctx, req)
}

// HealthCheck verifies the connection to the Orbi agent
func (c *CalendarGRPCClient) HealthCheck(ctx context.Context) error {
	if c.conn == nil {
		return fmt.Errorf("grpc connection is nil")
	}

	// Fast path: already ready
	if c.conn.GetState() == connectivity.Ready {
		return nil
	}

	// Wait a short time (bounded by ctx) for the connection to become Ready
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	state := c.conn.GetState()
	for {
		// If already ready after updates
		if c.conn.GetState() == connectivity.Ready {
			return nil
		}

		// Wait for a state change or context timeout/cancel
		ok := c.conn.WaitForStateChange(waitCtx, state)
		if !ok {
			// context done or timeout
			if err := waitCtx.Err(); err != nil {
				return fmt.Errorf("grpc health check timeout or canceled: %w", err)
			}
			break
		}

		// update observed state and loop
		state = c.conn.GetState()
	}

	return fmt.Errorf("grpc connection not ready: state=%v", c.conn.GetState())
}
