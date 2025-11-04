package grpc

import (
	"context"
	"fmt"

	pb "github.com/waydxd/Orbit-Orbi/proto/calendar"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
	"google.golang.org/grpc"
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
	
	conn, err := grpc.Dial(
		addr,
		grpc.WithInsecure(),
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
	_, err := c.conn.State()
	return err
}

