package gossip

import (
	"fmt"
	"time"
)

/**
Cassandra's GMS (Gossip Membership Service) is responsible for:
- Gossip protocol
- Membership management
- Node liveness tracking
- Heartbeat/state dissemination
- Failure detection (phi accrual)
- Managing endpoint states & application states

My Application State needs to answer 3 questions:
1. Who are the nodes? (membership list)
2. Are they alive? (Liveness)
3. How do I contact them? (Addressability)

Discovery: GossipState.StateByNode
Liveness: GossipState.StateByNode.Heartbeat.Generation
Addressability: GossipState.StateByNode.AppStates[AppHeartbeat].Value

ReferencePaper: https://iopscience.iop.org/article/10.1088/1742-6596/1437/1/012001/pdf
ReferenceCode: https://github.com/apache/cassandra/blob/trunk/src/java/org/apache/cassandra/gms/Gossiper.java

Overview:
	State Models (What we know about each node):
		HeartbeatState:
			Fields:
				Generation (int64) - Node's "Birth" time (unix seconds)
				Version (int64) - Monotonically incremented on each heartbeat
			Used for:
				Detecting node restarts (generation jumps)
				Detecting freshness (version increases)
		VersionedValue:
			Represents a versioned piece of application state.
			Fields:
				Version (int64) - Monotonically incremented on each update of the value
				Value (string) - Application state enum value
		ApplicationState:
			Represents a piece of application state.
			Fields:
				Version (int64) - Monotonically incremented on each update of the value
				Value (string) - Application state enum value
		EndpointState:

		GossipState:
			Fields:
				StateByNode (Map[NodeID]*EndpointState) - A map of all nodes and their endpoint states
			Used for:
				Maintaining the complete state of all nodes in the cluster
				Providing a single snapshot of all state for all nodes
	Gossiper:
		Central Engine responsible for gossiping information about the local endpoint.
			Fields:
				EndpointStateMap (Map[NodeAddress]*EndpointState) - A map of all nodes and their endpoint states
				LiveEndpoints (Set[NodeAddress]) - A set of all live nodes
				UnreachableEndpoints (Set[NodeAddress]) - A set of all endpoints not reachable by the local node
			Periodically (once a second):
				Updates local heartbeatState (incrementing version)
				Picks random peers to gossip with (live, unreachable, seeds)
				Executes a 3-step exchange:
					GOSSIP_DIGEST_SYN -> send digest list (endpoint, generation, maxVersion)
					GOSSIP_DIGEST_ACK -> Peer responds with "you're outdated on X, here's my newer state"
					GOSSIP_DIGEST_ACK2 -> Initiator sends remaining newer states back
				Merges remote EndpointStates into endpointStateMap using:
					(generation, version) comparison for heartbeats
					per-key version comparison for app states
			It also notifies:
				FailureDetector about fresh heartbeats
				Various Listeners when nodes join/leave/change status (IFailureDetectionEventListener)
			Used for:
				Maintaining the complete state of all nodes in the cluster
				Providing a single snapshot of all state for all nodes
				Responsible for gossiping information about the local endpoint.
				Maintains a list of live and dead endpoints.
	FailureDetector:
		This is a phi-accrual failure detector.
		It tracks heartbeat inter-arrival times for each node and computes a "suspicion level" phi.
		If phi exceeds a threshold, the node is marked down.

		It receives heartbeat timestamps from the Gossiper (via report()-style calls)
		TODO: Maybe a channel?
		Maintains statistical history per node
		Tells Gossiper when it considers a node down or back up.
		Gossiper updates:
			EndpointState.isAlive
			membershipSets: liveEndpoints vs. unreachableEndpoints
		This is how Cassandra answers: "Are they alive?" beyond just "Haven't heard from them in a while"

		Version 1:
			Time-based liveness using LastSeen + suspectAfter/deadAfter
	Integration/Listeners:
		IFailureDetectionEventListener - Allows other components to react to node status changes
		GossiperDiagnostics, GossiperEvent - Introspection / debugging for gossip
		StorageService - Uses gossip to know the ring, endpoints, and where replicas live.

File Organization:
	gossip.go - Core GossipState struct and constructor
	types.go - Basic type definitions (NodeID, AppStateKey, AppState)
	heartbeat_state.go - HeartbeatState management
	endpoint_state.go - EndpointState struct
	digest.go - Digest creation and comparison logic
	state_management.go - State merging and update logic
*/

// GossipState is the central state manager for the gossip protocol
// It maintains knowledge about all nodes in the cluster and coordinates gossip exchange
type GossipState struct {
	nodeID            NodeID
	clusterID         string
	heartbeatInterval time.Duration
	myHeartbeatState  *HeartbeatState // pointer to avoid copying mutex

	StateByNode map[NodeID]*EndpointState
	logFn       func(format string, args ...interface{})
}

// LocalHeartbeat returns a snapshot of the local node's heartbeat state
func (g *GossipState) LocalHeartbeat() HeartbeatStateSnapshot {
	if g.myHeartbeatState == nil {
		panic("GossipState not initialized: use NewGossipState")
	}
	// HeartbeatState manages its own mutex, so we can safely get a snapshot
	return g.myHeartbeatState.GetSnapshot()
}

func NewGossipState(nodeID NodeID, clusterID string, interval time.Duration, logFn func(format string, args ...interface{})) (*GossipState, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("interval must be greater than 0")
	}

	if nodeID == "" {
		return nil, fmt.Errorf("nodeID must be set")
	}

	if clusterID == "" {
		return nil, fmt.Errorf("clusterID must be set")
	}

	now := time.Now().Unix()
	myHeartbeatState := NewHeartbeatState(nodeID, now)
	initialSnapshot := myHeartbeatState.GetSnapshot()

	stateByNode := make(map[NodeID]*EndpointState)

	// Initialize local node's EndpointState in StateByNode - Cassandra style
	// This allows us to treat all nodes uniformly in CreateDigests and other operations
	stateByNode[nodeID] = &EndpointState{
		HeartbeatState:    initialSnapshot,
		applicationStates: make(map[AppStateKey]AppState),
		isAlive:           true,
		updateTimestamp:   now,
	}

	// Initialize default application states for local node
	stateByNode[nodeID].applicationStates[AppStatus] = AppState{
		Status:  "UP",
		Address: "",
		Value:   "UP",
		Version: 1,
	}
	stateByNode[nodeID].applicationStates[AppHeartbeat] = AppState{
		Status:  "UP",
		Address: "",
		Value:   "",
		Version: 1,
	}

	return &GossipState{
		nodeID:            nodeID,
		clusterID:         clusterID,
		heartbeatInterval: interval,
		myHeartbeatState:  myHeartbeatState,
		StateByNode:       stateByNode,
		logFn:             logFn,
	}, nil
}

// ClusterID returns the cluster ID
func (g *GossipState) ClusterID() string {
	return g.clusterID
}
