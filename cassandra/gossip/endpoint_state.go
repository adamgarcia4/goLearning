package gossip

/**
This is the per-node snapshot that ties everything together.
Represents both the heartbeat state and the ApplicationState in an EndpointState object.
Any state for a given endpoint can be retrieved from the EndpointState object.
Fields:
	HeartbeatState - The heartbeat state of the node
	ApplicationState (Map[ApplicationState]VersionedValue)
		A map of application states for the node
	Liveness Metadata:
		isAlive (bool) - Whether the node is alive
		updateTimestamp (int64) - Last we heard from this node
		// phi (float64) - Failure detection metric (phi accrual)
Used for:
	Storing the heartbeat state and application states for the node
	Tracking liveness metadata
	Providing a single snapshot of all state for a given endpoint



*/

type EndpointState struct {
	HeartbeatState    HeartbeatStateSnapshot // snapshot is safe to copy and store
	applicationStates map[AppStateKey]AppState

	isAlive         bool
	updateTimestamp int64
	// phi (float64) - Failure detection metric (phi accrual)
	// phi float64
}
