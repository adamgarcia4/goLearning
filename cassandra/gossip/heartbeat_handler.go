package gossip

import (
	"context"
	"encoding/json"
	"time"

	"github.com/adamgarcia4/goLearning/cassandra/logger"
)

/*
Heartbeat Handling

Heartbeats are the fundamental mechanism for:
1. Liveness detection - proving a node is still alive
2. State dissemination - piggybacking state updates on heartbeat messages
3. Version tracking - incrementing version counters to show freshness

The heartbeat system works in two directions:
- Outbound: Periodically send heartbeats with updated version numbers
- Inbound: Receive and process heartbeats from other nodes

Each heartbeat carries:
- Generation (node start time) - detects restarts
- Version (counter) - proves freshness
- Full state snapshot - for state synchronization
*/

// HeartbeatSender is a function that sends a heartbeat and returns the response node ID and timestamp
type HeartbeatSender func(heartbeatState HeartbeatStateSnapshot) (string, int64, error)

// SendHeartbeat updates the local heartbeat state and sends it via the provided sender function
// This is the main outbound heartbeat mechanism
func (g *GossipState) SendHeartbeat(sendHeartbeat HeartbeatSender) (string, int64, error) {
	if g.myHeartbeatState == nil {
		panic("GossipState not initialized: use NewGossipState")
	}
	// HeartbeatState manages its own mutex, so we don't need to lock GossipState here
	updatedHeartbeatState := g.myHeartbeatState.UpdateHeartbeat()

	// Update local node's EndpointState in StateByNode to keep it in sync
	g.updateLocalEndpointState(updatedHeartbeatState)

	// Serialize the StateByNode map to a string
	serializedState, err := json.Marshal(g.StateByNode)
	if err != nil {
		g.logFn("Failed to serialize StateByNode: %v", err)
	}
	g.logFn("Application State12: %s", string(serializedState))
	return sendHeartbeat(updatedHeartbeatState)
}

// InitializeHeartbeatSending starts a goroutine that periodically sends heartbeats
// This runs continuously until the context is cancelled
func (g *GossipState) InitializeHeartbeatSending(ctx context.Context, sendHeartbeat HeartbeatSender) {
	ticker := time.NewTicker(g.heartbeatInterval)
	defer ticker.Stop()
	logger.Printf("Node %s: Starting to send heartbeats every %v", string(g.nodeID), g.heartbeatInterval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _, err := g.SendHeartbeat(sendHeartbeat)
			if err != nil {
				g.logFn("Failed to send heartbeat: %v", err)
			}
		}
	}
}

// HandleHeartbeat processes an incoming heartbeat from a remote node
// It merges the remote state and returns the local node's heartbeat state
func (g *GossipState) HandleHeartbeat(remoteNodeID string, remoteGeneration int64, remoteVersion int64) (localNodeID string, localGeneration int64, localVersion int64, err error) {
	if g.myHeartbeatState == nil {
		panic("GossipState not initialized: use NewGossipState")
	}
	// TODO: Implement proper state merging logic
	// For now, just return our local state
	// In the future, this should:
	// 1. Compare remote generation/version with local state
	// 2. Merge remote state into StateByNode map
	// 3. Update local state if remote is newer

	snapshot := g.myHeartbeatState.GetSnapshot()
	g.logFn("Processing heartbeat from %s (remote gen: %d, local gen: %d)", remoteNodeID, remoteGeneration, snapshot.Generation)
	return string(snapshot.NodeID), snapshot.Generation, snapshot.Version, nil
}

// Start begins the heartbeat sending process in a background goroutine
func (g *GossipState) Start(ctx context.Context, sendHeartbeat HeartbeatSender) {
	go g.InitializeHeartbeatSending(ctx, sendHeartbeat)
}
