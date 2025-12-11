package gossip

import (
	"context"
	"fmt"
	"time"

	"github.com/adamgarcia4/goLearning/cassandra/logger"
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
*/

type GossipState struct {
	nodeID            NodeID
	heartbeatInterval time.Duration
	myHeartbeatState  *HeartbeatState // pointer to avoid copying mutex

	// StateByNode map[NodeID]*EndpointState
}

// HeartbeatSender is a function that sends a heartbeat and returns the response node ID and timestamp
type HeartbeatSender func(heartbeatState HeartbeatStateSnapshot) (string, int64, error)

func (g *GossipState) SendHeartbeat(sendHeartbeat HeartbeatSender) (string, int64, error) {
	// HeartbeatState manages its own mutex, so we don't need to lock GossipState here
	updatedHeartbeatState := g.myHeartbeatState.UpdateHeartbeat()
	return sendHeartbeat(updatedHeartbeatState)
}

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
				logger.Printf("Node %s: Failed to send heartbeat: %v", string(g.nodeID), err)
			}
		}
	}
}

func (g *GossipState) LocalHeartbeat() HeartbeatStateSnapshot {
	// HeartbeatState manages its own mutex, so we can safely get a snapshot
	return g.myHeartbeatState.GetSnapshot()
}

// HandleHeartbeat processes an incoming heartbeat from a remote node
// It merges the remote state and returns the local node's heartbeat state
func (g *GossipState) HandleHeartbeat(remoteNodeID string, remoteGeneration int64, remoteVersion int64) (localNodeID string, localGeneration int64, localVersion int64, err error) {
	// TODO: Implement proper state merging logic
	// For now, just return our local state
	// In the future, this should:
	// 1. Compare remote generation/version with local state
	// 2. Merge remote state into StateByNode map
	// 3. Update local state if remote is newer

	snapshot := g.myHeartbeatState.GetSnapshot()
	return string(snapshot.NodeID), snapshot.Generation, snapshot.Version, nil
}

func (g *GossipState) Start(ctx context.Context, sendHeartbeat HeartbeatSender) {
	go g.InitializeHeartbeatSending(ctx, sendHeartbeat)
}

func NewGossipState(nodeID NodeID, interval time.Duration) (*GossipState, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("interval must be greater than 0")
	}

	if nodeID == "" {
		return nil, fmt.Errorf("nodeID must be set")
	}

	return &GossipState{
		nodeID:            nodeID,
		heartbeatInterval: interval,
		myHeartbeatState:  NewHeartbeatState(nodeID, time.Now().Unix()),
	}, nil
}
