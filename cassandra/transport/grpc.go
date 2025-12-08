package transport

import (
	"fmt"
	"net"
	"strings"

	gossipProtobuffer "github.com/adamgarcia4/goLearning/cassandra/api/gossip/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type GRPC struct {
	addr          string
	srv           *grpc.Server
	lis           net.Listener
	nodeID        string
	gossipHandler GossipHandler
	// logger *log.Logger
}

func (g *GRPC) setupTcp() (net.Listener, error) {
	lis, err := net.Listen("tcp", g.addr)

	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	return lis, nil
}

func (g *GRPC) setupServices() error {
	if g.gossipHandler == nil {
		return fmt.Errorf("gossip handler must be set")
	}

	heartbeatServer := &HeartbeatServiceServer{
		handler: g.gossipHandler,
		nodeID:  g.nodeID,
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
	if err := g.setupServices(); err != nil {
		return fmt.Errorf("failed to setup services: %w", err)
	}

	// Register reflection service for gRPC tools (grpcurl, grpcui, etc.)
	reflection.Register(g.srv)

	// Start serving (this blocks)
	return g.srv.Serve(g.lis)
}

func NewGRPC(addr string, nodeID string, gossipHandler GossipHandler) (*GRPC, error) {
	if addr == "" || !strings.Contains(addr, ":") {
		return nil, fmt.Errorf("invalid address: %s", addr)
	}

	if nodeID == "" {
		return nil, fmt.Errorf("nodeID must be provided")
	}

	if gossipHandler == nil {
		return nil, fmt.Errorf("gossip handler must be provided")
	}

	return &GRPC{
		addr:          addr,
		srv:           grpc.NewServer(),
		nodeID:        nodeID,
		gossipHandler: gossipHandler,
	}, nil
}
