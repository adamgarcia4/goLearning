package gossip

import "sync"

/*
*

Reference: https://github.com/apache/cassandra/blob/trunk/src/java/org/apache/cassandra/gms/HeartBeatState.java
*/

// HeartbeatStateSnapshot is a snapshot of HeartbeatState without the mutex.
// This type is safe to copy and send over the network.
type HeartbeatStateSnapshot struct {
	NodeID     NodeID
	Generation int64 // node start time (unix seconds)
	Version    int64 // incremented on each heartbeat
}

// HeartbeatState is the internal state with its own mutex for thread safety.
type HeartbeatState struct {
	mu         sync.RWMutex
	NodeID     NodeID
	Generation int64 // node start time (unix seconds)
	Version    int64 // incremented on each heartbeat
}

// UpdateHeartbeat increments the version and returns a snapshot of the current state
// (without the mutex) for sending over the network.
func (h *HeartbeatState) UpdateHeartbeat() HeartbeatStateSnapshot {
	h.mu.Lock()
	h.Version++
	// Capture values while holding the lock
	nodeID := h.NodeID
	generation := h.Generation
	version := h.Version
	h.mu.Unlock()

	// Return a snapshot without the mutex (safe to copy)
	return HeartbeatStateSnapshot{
		NodeID:     nodeID,
		Generation: generation,
		Version:    version,
	}
}

// GetSnapshot returns a snapshot of the current state (without the mutex) for reading.
func (h *HeartbeatState) GetSnapshot() HeartbeatStateSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return HeartbeatStateSnapshot{
		NodeID:     h.NodeID,
		Generation: h.Generation,
		Version:    h.Version,
	}
}

// GetVersion returns the current version in a thread-safe manner.
func (h *HeartbeatState) GetVersion() int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.Version
}

// GetGeneration returns the current generation in a thread-safe manner.
func (h *HeartbeatState) GetGeneration() int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.Generation
}

// GetNodeID returns the node ID in a thread-safe manner.
func (h *HeartbeatState) GetNodeID() NodeID {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.NodeID
}

func NewEmptyHeartbeatState(nodeID NodeID, generation int64) *HeartbeatState {
	return &HeartbeatState{
		NodeID:     nodeID,
		Generation: generation,
		Version:    0,
	}
}
