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

	// Manual heartbeat support
	manualSendHeartbeat gossip.HeartbeatSender // Stored for manual heartbeat triggering

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
}

// New creates a new node with the given configuration
func New(config *Config) (*Node, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Create gossip state
	gossipState, err := gossip.NewGossipState(config.NodeID, config.HeartbeatInterval, func(format string, args ...interface{}) {
		logger.Printf("[%s] %s", string(config.NodeID), fmt.Sprintf(format, args...))
	})
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
	if err := n.startClient(); err != nil {
		return fmt.Errorf("failed to start client: %w", err)
	}
	n.logf("Node %s will send heartbeats to %s every %v",
		n.config.NodeID, n.config.TargetServer, n.config.HeartbeatInterval)

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
	nodeID := n.config.NodeID
	grpcServer := n.grpcServer
	clientConn := n.clientConn

	// Cancel context to stop all goroutines (heartbeat sending, etc.)
	n.cancel()
	n.mu.Unlock()

	n.logf("Stopping node %s...", nodeID)

	// Stop gRPC server first (this will unblock the Serve() call)
	// Lock is released to avoid deadlocks if callbacks try to access Node
	if grpcServer != nil {
		if err := grpcServer.Stop(); err != nil {
			n.logf("Error stopping gRPC server: %v", err)
		}
	}

	// Close client connection if exists
	// Lock is released to avoid deadlocks if callbacks try to access Node
	if clientConn != nil {
		if err := clientConn.Close(); err != nil {
			n.logf("Error closing client connection: %v", err)
		}
	}

	n.logf("Node %s stopped", nodeID)
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

	n.logf("gRPC server starting on %s (node-id: %s)", n.config.GetAddress(), n.config.NodeID)

	// Start() performs binding synchronously and returns an error immediately if binding fails.
	// If binding succeeds, it spawns Serve in a goroutine and returns nil.
	// This ensures that binding errors (e.g., port already in use) are surfaced synchronously.
	if err := grpcTransport.Start(); err != nil {
		return fmt.Errorf("failed to bind gRPC server: %w", err)
	}

	// Binding succeeded - server is now serving in a background goroutine
	return nil
}

// startClient starts the client that sends heartbeats
func (n *Node) startClient() error {
	// Create gRPC client connection
	n.logf("Connecting to target server %s", n.config.TargetServer)
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
		n.logf("Sending heartbeat to %s", n.config.TargetServer)
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

	// Store for manual heartbeat mode
	n.manualSendHeartbeat = sendHeartbeat

	// Start automatic heartbeat sending only if not in manual mode
	if !n.config.ManualHeartbeat {
		n.gossipState.Start(n.ctx, sendHeartbeat)
		n.logf("Automatic heartbeat sending enabled (interval: %v)", n.config.HeartbeatInterval)
	} else {
		n.logf("Manual heartbeat mode enabled - press 'H' to send heartbeats")
	}

	return nil
}

// SendHeartbeat manually triggers a heartbeat (only works in manual heartbeat mode)
func (n *Node) SendHeartbeat() error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.manualSendHeartbeat == nil {
		return fmt.Errorf("heartbeat sender not initialized")
	}

	if !n.config.ManualHeartbeat {
		return fmt.Errorf("node is not in manual heartbeat mode")
	}

	// Send heartbeat using stored sender function
	_, _, err := n.gossipState.SendHeartbeat(n.manualSendHeartbeat)
	return err
}

// logf logs using the global logger (which handles both stdout and log buffer)
func (n *Node) logf(format string, args ...interface{}) {
	// Use logger with node ID as prefix
	logger.Printf("[%s] %s", string(n.config.NodeID), fmt.Sprintf(format, args...))
}
