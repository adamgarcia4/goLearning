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

// CompareDigests compares received digests with local state and determines:
// - endpointStates: States we have that the peer needs (we're newer)
// - requestDigests: States the peer has that we need (peer is newer)
//
// Algorithm follows GOSSIP_PROTOCOL_REFERENCE.md:
// FOR EACH remote digest:
//
//	IF local.generation > remote.generation → Send local state
//	ELSE IF local.generation < remote.generation → Request their state
//	ELSE (same generation):
//	  IF local.maxVersion > remote.maxVersion → Send local state
//	  ELSE IF local.maxVersion < remote.maxVersion → Request their state
//	  ELSE → No action (in sync)
//
// FOR EACH local node NOT in remote digests → Send local state
func (g *GossipState) CompareDigests(remoteDigests []GossipDigest) (
	endpointStates []*EndpointState,
	requestDigests []GossipDigest,
) {
	// Track which remote nodes we've seen in the digest
	seenRemoteNodes := make(map[NodeID]bool)
	endpointStates = make([]*EndpointState, 0)
	requestDigests = make([]GossipDigest, 0)

	// Compare each remote digest with local state
	for _, remoteDigest := range remoteDigests {
		nodeID := NodeID(remoteDigest.NodeID)
		seenRemoteNodes[nodeID] = true

		localState := g.StateByNode[nodeID]

		if localState == nil {
			// We don't know about this node - request it
			requestDigests = append(requestDigests, remoteDigest)
			continue
		}

		// Here, I have a remoteDigest and a local. I need to compare.
		localGen := localState.HeartbeatState.Generation
		localMaxVer := g.calculateMaxVersionForEndpoint(localState)
		remoteGen := int64(remoteDigest.Generation)
		remoteMaxVer := int64(remoteDigest.MaxVersion)

		// Compare generations first (higher generation = newer/restart)
		if localGen > remoteGen {
			// We have newer generation - send our state
			endpointStates = append(endpointStates, localState)
		} else if localGen < remoteGen {
			// Peer has newer generation - request their state
			requestDigests = append(requestDigests, remoteDigest)
		} else {
			// Same generation - compare max versions
			if localMaxVer > remoteMaxVer {
				// We have newer version - send our state
				endpointStates = append(endpointStates, localState)
			} else if localMaxVer < remoteMaxVer {
				// Peer has newer version - request their state
				requestDigests = append(requestDigests, remoteDigest)
			}
			// If equal, we're in sync - no action needed
		}
	}

	// Check for nodes we know about that peer didn't mention
	for nodeID, localState := range g.StateByNode {
		if !seenRemoteNodes[nodeID] {
			// Peer doesn't know about this node - send it
			endpointStates = append(endpointStates, localState)
		}
	}

	return endpointStates, requestDigests
}
