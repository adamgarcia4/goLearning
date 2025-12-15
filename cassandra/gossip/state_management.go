package gossip

import "time"

/*
State Management and Merging

This file handles state updates and merging for both local and remote nodes.

State merging is critical in gossip protocols because:
1. Nodes exchange state information periodically
2. States must be reconciled using version vectors (generation, version)
3. Newer states override older ones
4. Application states are merged per-key based on their individual versions

The gossip protocol uses:
- Generation for detecting node restarts (higher generation wins)
- Version for detecting freshness within same generation (higher version wins)
- Per-key versions for application states
*/

// refreshNodeState updates the state for a node with new heartbeat information
// It updates the HeartbeatState and AppState entries, incrementing versions appropriately
//
// This is called when receiving gossip from remote nodes to update our view of their state
func (g *GossipState) refreshNodeState(nodeID NodeID, heartbeatState HeartbeatStateSnapshot) {
	now := time.Now().Unix()

	if g.StateByNode[nodeID] == nil {
		// Create new EndpointState
		g.StateByNode[nodeID] = &EndpointState{
			HeartbeatState:    heartbeatState,
			applicationStates: make(map[AppStateKey]AppState),
			isAlive:           true,
			updateTimestamp:   now,
		}

		// Initialize AppState entries with default values
		g.StateByNode[nodeID].applicationStates[AppStatus] = AppState{
			Status:  "UP",
			Address: "",
			Value:   "UP",
			Version: 1,
		}

		// AppHeartbeat will be set when we know the address
		// For now, initialize with empty address
		g.StateByNode[nodeID].applicationStates[AppHeartbeat] = AppState{
			Status:  "UP",
			Address: "",
			Value:   "", // Address will be set when known
			Version: 1,
		}
	} else {
		// Update existing EndpointState
		endpointState := g.StateByNode[nodeID]

		// Update HeartbeatState
		endpointState.HeartbeatState = heartbeatState

		// Update AppStatus - increment version if status changed, otherwise just update timestamp
		appStatus, exists := endpointState.applicationStates[AppStatus]
		if !exists {
			appStatus = AppState{
				Status:  "UP",
				Address: "",
				Value:   "UP",
				Version: 1,
			}
		} else {
			// Increment version to indicate state refresh
			appStatus.Version++
			appStatus.Status = "UP"
			appStatus.Value = "UP"
		}
		endpointState.applicationStates[AppStatus] = appStatus

		// Update AppHeartbeat - preserve existing address if set, increment version
		appHeartbeat, exists := endpointState.applicationStates[AppHeartbeat]
		if !exists {
			appHeartbeat = AppState{
				Status:  "UP",
				Address: "",
				Value:   "",
				Version: 1,
			}
		} else {
			// Increment version to indicate state refresh
			appHeartbeat.Version++
			appHeartbeat.Status = "UP"
			// Preserve existing address/Value if it was set
		}
		endpointState.applicationStates[AppHeartbeat] = appHeartbeat

		// Update liveness metadata
		endpointState.isAlive = true
		endpointState.updateTimestamp = now
	}
}

// updateLocalEndpointState keeps the local node's EndpointState in StateByNode synchronized
//
// This is called after each local heartbeat update to ensure the local node's state
// in StateByNode stays current. This allows the local node to be treated uniformly
// with remote nodes in operations like CreateDigests()
func (g *GossipState) updateLocalEndpointState(snapshot HeartbeatStateSnapshot) {
	now := time.Now().Unix()

	if localState := g.StateByNode[g.nodeID]; localState != nil {
		// Update existing local state
		localState.HeartbeatState = snapshot
		localState.isAlive = true
		localState.updateTimestamp = now

		// Increment app state versions to reflect the heartbeat update
		for key, appState := range localState.applicationStates {
			appState.Version++
			localState.applicationStates[key] = appState
		}
	}
}
