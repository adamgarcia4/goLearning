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

	// Peer connections for gossip
	peerConns   map[string]*grpc.ClientConn
	peerClients map[string]pbproto.GossipServiceClient

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
	gossipState, err := gossip.NewGossipState(config.NodeID, config.ClusterID, config.HeartbeatInterval, func(format string, args ...interface{}) {
		logger.Printf("[%s] %s", string(config.NodeID), fmt.Sprintf(format, args...))
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gossip state: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Node{
		config:      config,
		gossipState: gossipState,
		peerConns:   make(map[string]*grpc.ClientConn),
		peerClients: make(map[string]pbproto.GossipServiceClient),
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

// Start starts the node (both server and gossip connections)
func (n *Node) Start() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Always start the server first
	if err := n.startServer(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Connect to seed peers
	if err := n.connectToPeers(); err != nil {
		return fmt.Errorf("failed to connect to peers: %w", err)
	}

	// Start gossip round if not in manual mode
	if !n.config.ManualHeartbeat && len(n.peerClients) > 0 {
		n.startGossipRound()
	}

	n.logf("Node %s started on %s (cluster: %s, seeds: %v)",
		n.config.NodeID, n.config.GetAddress(), n.config.ClusterID, n.config.Seeds)
	return nil
}

// Stop stops the node gracefully
func (n *Node) Stop() error {
	n.mu.Lock()
	nodeID := n.config.NodeID
	grpcServer := n.grpcServer
	peerConns := n.peerConns

	// Cancel context to stop all goroutines (gossip rounds, etc.)
	n.cancel()
	n.mu.Unlock()

	n.logf("Stopping node %s...", nodeID)

	// Stop gRPC server first (this will unblock the Serve() call)
	if grpcServer != nil {
		if err := grpcServer.Stop(); err != nil {
			n.logf("Error stopping gRPC server: %v", err)
		}
	}

	// Close all peer connections
	for addr, conn := range peerConns {
		if err := conn.Close(); err != nil {
			n.logf("Error closing connection to %s: %v", addr, err)
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
		n.config.ClusterID,
		n.gossipState,
		n.AddPeer, // Wire peer discovery callback
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

// connectToPeers establishes connections to all configured seed nodes
func (n *Node) connectToPeers() error {
	myAddr := n.config.GetAddress()

	for _, seedAddr := range n.config.Seeds {
		// Skip self
		if seedAddr == myAddr {
			continue
		}

		n.logf("Connecting to seed peer %s", seedAddr)
		conn, err := grpc.NewClient(
			seedAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			n.logf("Warning: failed to connect to seed %s: %v", seedAddr, err)
			continue // Don't fail entirely if one seed is unreachable
		}

		n.peerConns[seedAddr] = conn
		n.peerClients[seedAddr] = pbproto.NewGossipServiceClient(conn)
		n.logf("Connected to peer %s", seedAddr)
	}

	return nil
}

// startGossipRound starts the periodic gossip round that sends SYN to all peers
func (n *Node) startGossipRound() {
	// Create sender function that sends SYN to all peers
	sendSynToAll := func(digests []gossip.GossipDigest) {
		// Convert gossip.GossipDigest to proto
		protoDigests := make([]*pbproto.GossipDigest, 0, len(digests))

		for _, d := range digests {
			protoDigests = append(protoDigests, &pbproto.GossipDigest{
				NodeId:     d.NodeID,
				Generation: int64(d.Generation),
				MaxVersion: int64(d.MaxVersion),
			})
		}

		msg := &pbproto.GossipDigestSynMsg{
			ClusterId:     n.config.ClusterID,
			Digests:       protoDigests,
			SenderAddress: n.config.GetAddress(),
		}

		// Send to all peers concurrently
		for addr, client := range n.peerClients {
			go func(addr string, client pbproto.GossipServiceClient) {
				resp, err := client.GossipDigestSyn(n.ctx, msg)
				if err != nil {
					n.logf("Failed to send SYN to %s: %v", addr, err)
					return
				}
				// TODO: Process ACK response (Phase 2)
				n.logf("Received ACK from %s with %d endpoint states and %d request digests",
					addr, len(resp.EndpointStates), len(resp.RequestDigests))
			}(addr, client)
		}
	}

	// Start the gossip round loop
	n.gossipState.StartGossipRound(n.ctx, sendSynToAll)
	n.logf("Gossip round started (interval: %v)", n.config.HeartbeatInterval)
}

// SendGossipRound manually triggers a gossip round (only works in manual mode)
func (n *Node) SendGossipRound() error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if !n.config.ManualHeartbeat {
		return fmt.Errorf("node is not in manual gossip mode")
	}

	if len(n.peerClients) == 0 {
		return fmt.Errorf("no peers connected")
	}

	// Create and send digests
	digests := n.gossipState.CreateDigests()

	// Convert and send
	protoDigests := make([]*pbproto.GossipDigest, 0, len(digests))
	for _, d := range digests {
		protoDigests = append(protoDigests, &pbproto.GossipDigest{
			NodeId:     d.NodeID,
			Generation: int64(d.Generation),
			MaxVersion: int64(d.MaxVersion),
		})
	}

	msg := &pbproto.GossipDigestSynMsg{
		ClusterId:     n.config.ClusterID,
		Digests:       protoDigests,
		SenderAddress: n.config.GetAddress(),
	}

	for addr, client := range n.peerClients {
		go func(addr string, client pbproto.GossipServiceClient) {
			resp, err := client.GossipDigestSyn(n.ctx, msg)
			if err != nil {
				n.logf("Failed to send SYN to %s: %v", addr, err)
				return
			}
			n.logf("Received ACK from %s with %d endpoint states", addr, len(resp.EndpointStates))
		}(addr, client)
	}

	return nil
}

// AddPeer dynamically adds a new peer connection if not already connected.
// This enables Cassandra-style peer discovery through gossip.
func (n *Node) AddPeer(peerAddr string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Skip if already connected
	if _, exists := n.peerConns[peerAddr]; exists {
		return nil
	}

	// Skip self
	if peerAddr == n.config.GetAddress() {
		return nil
	}

	conn, err := grpc.NewClient(
		peerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to peer %s: %w", peerAddr, err)
	}

	n.peerConns[peerAddr] = conn
	n.peerClients[peerAddr] = pbproto.NewGossipServiceClient(conn)
	n.logf("Discovered and connected to new peer: %s", peerAddr)
	return nil
}

// logf logs using the global logger (which handles both stdout and log buffer)
func (n *Node) logf(format string, args ...interface{}) {
	// Use logger with node ID as prefix
	logger.Printf("[%s] %s", string(n.config.NodeID), fmt.Sprintf(format, args...))
}
