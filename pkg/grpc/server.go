package grpc

import (
	"fmt"
	"net"

	"github.com/waydxd/Orbit-core/pkg/logger"
	"google.golang.org/grpc"
)

// UnaryServerInterceptor is an alias for grpc.UnaryServerInterceptor
type UnaryServerInterceptor = grpc.UnaryServerInterceptor

// Server represents the gRPC server for Core to expose services to Agent
type Server struct {
	server   *grpc.Server
	listener net.Listener
	logger   *logger.Logger
	port     int
}

// ServerConfig holds the configuration for the gRPC server
type ServerConfig struct {
	Port         int
	Interceptors []grpc.UnaryServerInterceptor
}

// NewServer creates a new gRPC server
func NewServer(cfg ServerConfig, log *logger.Logger) (*Server, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		log.Error("Failed to create listener", "port", cfg.Port, "error", err)
		return nil, err
	}

	// Create server with interceptors if provided
	var opts []grpc.ServerOption
	if len(cfg.Interceptors) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(cfg.Interceptors...))
	}
	grpcServer := grpc.NewServer(opts...)

	return &Server{
		server:   grpcServer,
		listener: listener,
		logger:   log,
		port:     cfg.Port,
	}, nil
}

// RegisterService registers a gRPC service with the server
func (s *Server) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	s.server.RegisterService(desc, impl)
}

// Underlying returns the underlying *grpc.Server from google.golang.org/grpc
func (s *Server) Underlying() *grpc.Server {
	return s.server
}

// Start starts the gRPC server
func (s *Server) Start() error {
	s.logger.Info("Starting gRPC server", "port", s.port)
	return s.server.Serve(s.listener)
}

// Stop gracefully stops the gRPC server
func (s *Server) Stop() {
	s.logger.Info("Stopping gRPC server")
	s.server.GracefulStop()
}

// GetPort returns the port the server is listening on
func (s *Server) GetPort() int {
	return s.port
}
