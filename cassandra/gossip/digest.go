package gossip

/*
Digest Creation and Management

In Cassandra's gossip protocol, digests are compact summaries of node states
used in the 3-phase gossip exchange:

	GOSSIP_DIGEST_SYN -> send digest list (endpoint, generation, maxVersion)
	GOSSIP_DIGEST_ACK -> Peer responds with "you're outdated on X, here's my newer state"
	GOSSIP_DIGEST_ACK2 -> Initiator sends remaining newer states back

Digests allow nodes to efficiently determine which state updates they need
to exchange without sending full state information upfront.
*/

// GossipDigest is a compact summary of a node's state (generation + max version)
// Used in the gossip protocol's SYN phase to compare states
type GossipDigest struct {
	NodeID     string
	Generation int
	MaxVersion int
}

// CreateDigests generates digest summaries for all known nodes (local and remote)
// This matches Cassandra's approach of treating the local node uniformly with remote nodes
func (g *GossipState) CreateDigests() []GossipDigest {
	digests := make([]GossipDigest, 0, len(g.StateByNode))

	// Iterate through all nodes (local and remote) - Cassandra style
	// local epstate will be part of StateByNode
	for nodeID, endpointState := range g.StateByNode {
		digests = append(digests, GossipDigest{
			NodeID:     string(nodeID),
			Generation: int(endpointState.HeartbeatState.Generation),
			MaxVersion: int(g.calculateMaxVersionForEndpoint(endpointState)),
		})
	}
	return digests
}

// calculateMaxVersionForEndpoint finds the highest version across heartbeat and app states
// This matches Cassandra's getMaxEndpointStateVersion() method
func (g *GossipState) calculateMaxVersionForEndpoint(ep *EndpointState) int64 {
	maxVer := ep.HeartbeatState.Version

	for _, appState := range ep.applicationStates {
		if appState.Version > maxVer {
			maxVer = appState.Version
		}
	}

	return maxVer
}
