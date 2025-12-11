package node

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pbproto "github.com/adamgarcia4/goLearning/cassandra/api/gossip/v1"
	"github.com/adamgarcia4/goLearning/cassandra/gossip"
	"github.com/adamgarcia4/goLearning/cassandra/logger"
	"github.com/adamgarcia4/goLearning/cassandra/transport"
)

// Node represents a gossip protocol node
type Node struct {
	config      *Config
	gossipState *gossip.GossipState
	grpcServer  *transport.GRPC
	clientConn  *grpc.ClientConn

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
}

// New creates a new node with the given configuration
func New(config *Config) (*Node, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Create gossip state
	gossipState, err := gossip.NewGossipState(config.NodeID, config.HeartbeatInterval)
	if err != nil {
		return nil, fmt.Errorf("failed to create gossip state: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Node{
		config:      config,
		gossipState: gossipState,
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

// Start starts the node (both server and client if configured)
func (n *Node) Start() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Start client mode if configured
	if n.config.ClientMode {
		if err := n.startClient(); err != nil {
			return fmt.Errorf("failed to start client: %w", err)
		}
		n.logf("Client mode enabled: node %s will send heartbeats to %s every %v",
			n.config.NodeID, n.config.TargetServer, n.config.HeartbeatInterval)
	}

	// Always start the server
	if err := n.startServer(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	n.logf("Node %s started on %s", n.config.NodeID, n.config.GetAddress())
	return nil
}

// Stop stops the node gracefully
func (n *Node) Stop() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.logf("Stopping node %s...", n.config.NodeID)

	// Cancel context to stop all goroutines (heartbeat sending, etc.)
	n.cancel()

	// Stop gRPC server first (this will unblock the Serve() call)
	if n.grpcServer != nil {
		n.grpcServer.Stop()
	}

	// Close client connection if exists
	if n.clientConn != nil {
		if err := n.clientConn.Close(); err != nil {
			n.logf("Error closing client connection: %v", err)
		}
	}

	// Wait for all goroutines to finish
	n.wg.Wait()

	n.logf("Node %s stopped", n.config.NodeID)
	return nil
}

// GetGossipState returns the gossip state (for external access)
func (n *Node) GetGossipState() *gossip.GossipState {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.gossipState
}

// GetConfig returns the node configuration (for external access)
func (n *Node) GetConfig() *Config {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.config
}

// startServer starts the gRPC server
func (n *Node) startServer() error {
	grpcTransport, err := transport.NewGRPC(
		n.config.GetAddress(),
		string(n.config.NodeID),
		n.gossipState,
	)
	if err != nil {
		return fmt.Errorf("failed to create gRPC transport: %w", err)
	}

	n.grpcServer = grpcTransport

	// Start server in a goroutine
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		n.logf("gRPC server starting on %s (node-id: %s)", n.config.GetAddress(), n.config.NodeID)
		if err := grpcTransport.Start(); err != nil {
			n.logf("gRPC server error: %v", err)
		}
	}()

	return nil
}

// startClient starts the client that sends heartbeats
func (n *Node) startClient() error {
	// Create gRPC client connection
	conn, err := grpc.NewClient(
		n.config.TargetServer,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to target server: %w", err)
	}

	n.clientConn = conn
	client := pbproto.NewHeartbeatServiceClient(conn)

	// Create heartbeat sender function
	sendHeartbeat := func(heartbeatState gossip.HeartbeatStateSnapshot) (string, int64, error) {
		req := &pbproto.HeartbeatRequest{
			NodeId:    string(heartbeatState.NodeID),
			Timestamp: heartbeatState.Generation,
		}

		resp, err := client.Heartbeat(n.ctx, req)
		if err != nil {
			return "", 0, err
		}

		return resp.NodeId, resp.Timestamp, nil
	}

	// Start heartbeat sending
	n.gossipState.Start(n.ctx, sendHeartbeat)

	return nil
}

// logf logs using the global logger (which handles both stdout and log buffer)
func (n *Node) logf(format string, args ...interface{}) {
	// Use logger with node ID as prefix
	logger.Printf("[%s] %s", string(n.config.NodeID), fmt.Sprintf(format, args...))
}

