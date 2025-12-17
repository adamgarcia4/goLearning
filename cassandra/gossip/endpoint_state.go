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

// GetApplicationStates returns a copy of the application states map
func (es *EndpointState) GetApplicationStates() map[AppStateKey]AppState {
	result := make(map[AppStateKey]AppState)
	for k, v := range es.applicationStates {
		result[k] = v
	}
	return result
}

// GetIsAlive returns whether the node is alive
func (es *EndpointState) GetIsAlive() bool {
	return es.isAlive
}

// GetUpdateTimestamp returns the update timestamp
func (es *EndpointState) GetUpdateTimestamp() int64 {
	return es.updateTimestamp
}

// NewEndpointState creates a new EndpointState from components
// This is used when converting from proto format
func NewEndpointState(
	heartbeatState HeartbeatStateSnapshot,
	appStates map[AppStateKey]AppState,
	isAlive bool,
	updateTimestamp int64,
) *EndpointState {
	if appStates == nil {
		appStates = make(map[AppStateKey]AppState)
	}
	return &EndpointState{
		HeartbeatState:    heartbeatState,
		applicationStates: appStates,
		isAlive:           isAlive,
		updateTimestamp:   updateTimestamp,
	}
}
