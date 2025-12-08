package transport

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	gossipProtobuffer "github.com/adamgarcia4/goLearning/cassandra/api/gossip/v1" // Import to register proto file descriptors for reflection
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type GRPC struct {
	addr   string
	srv    *grpc.Server
	lis    net.Listener
	nodeID string
	// logger *log.Logger
	// eng    *node.GossipEngine
}

func (g *GRPC) setupTcp() (net.Listener, error) {
	lis, err := net.Listen("tcp", g.addr)

	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	return lis, nil
}

// HeartbeatServiceServer implements the gRPC HeartbeatService
type HeartbeatServiceServer struct {
	gossipProtobuffer.UnimplementedHeartbeatServiceServer
	nodeID string
}

// Heartbeat handles heartbeat requests
func (s *HeartbeatServiceServer) Heartbeat(ctx context.Context, req *gossipProtobuffer.HeartbeatRequest) (*gossipProtobuffer.HeartbeatResponse, error) {
	// TODO: Integrate with gossip engine to merge state
	// For now, just return a response with our node ID and current timestamp
	return &gossipProtobuffer.HeartbeatResponse{
		NodeId:    s.nodeID,
		Timestamp: time.Now().Unix(),
	}, nil
}

func (g *GRPC) setupServices(nodeID string) error {
	heartbeatServer := &HeartbeatServiceServer{
		nodeID: nodeID,
	}
	gossipProtobuffer.RegisterHeartbeatServiceServer(g.srv, heartbeatServer)
	return nil
}

func (g *GRPC) Start() error {
	if lis, err := g.setupTcp(); err != nil {
		return fmt.Errorf("failed to setup TCP: %w", err)
	} else {
		g.lis = lis
	}

	// Register services
	if err := g.setupServices(g.nodeID); err != nil {
		return fmt.Errorf("failed to setup services: %w", err)
	}

	// Register reflection service for gRPC tools (grpcurl, grpcui, etc.)
	reflection.Register(g.srv)

	// Start serving (this blocks)
	return g.srv.Serve(g.lis)
}

func NewGRPC(addr string, nodeID string) (*GRPC, error) {
	if addr == "" || !strings.Contains(addr, ":") {
		return nil, fmt.Errorf("invalid address: %s", addr)
	}

	if nodeID == "" {
		return nil, fmt.Errorf("nodeID must be provided")
	}

	return &GRPC{
		addr:   addr,
		srv:    grpc.NewServer(),
		nodeID: nodeID,
	}, nil
}
