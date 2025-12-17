package transport

import (
	"context"
	"time"

	gossipProtobuffer "github.com/adamgarcia4/goLearning/cassandra/api/gossip/v1" // Import to register proto file descriptors for reflection
	"github.com/adamgarcia4/goLearning/cassandra/gossip"
	"github.com/adamgarcia4/goLearning/cassandra/logger"
)

type GossipHandler interface {
	HandleHeartbeat(remoteNodeID string, remoteGeneration int64, remoteVersion int64) (localNodeID string, localGeneration int64, localVersion int64, err error)
}

type HeartbeatServiceServer struct {
	gossipProtobuffer.UnimplementedHeartbeatServiceServer
	handler GossipHandler
	nodeID  string
}

// GossipDigestHandler handles incoming gossip digest messages
type GossipDigestHandler interface {
	// HandleGossipSyn processes an incoming SYN message and returns ACK components
	// All types are gossip package types (not proto)
	HandleGossipSyn(clusterID string, digests []gossip.GossipDigest) (
		endpointStates map[gossip.NodeID]*gossip.EndpointState,
		requestDigests []gossip.GossipDigest,
		err error,
	)
}

// PeerDiscoveryCallback is called when a new peer is discovered through gossip
type PeerDiscoveryCallback func(peerAddr string) error

// protoDigestToGossip converts proto GossipDigest to gossip GossipDigest
func protoDigestToGossip(d *gossipProtobuffer.GossipDigest) gossip.GossipDigest {
	return gossip.GossipDigest{
		NodeID:     d.NodeId,
		Generation: int(d.Generation),
		MaxVersion: int(d.MaxVersion),
	}
}

// gossipDigestToProto converts gossip GossipDigest to proto GossipDigest
func gossipDigestToProto(d gossip.GossipDigest) *gossipProtobuffer.GossipDigest {
	return &gossipProtobuffer.GossipDigest{
		NodeId:     d.NodeID,
		Generation: int64(d.Generation),
		MaxVersion: int64(d.MaxVersion),
	}
}

// endpointStateToProto converts gossip EndpointState to proto EndpointState
func endpointStateToProto(es *gossip.EndpointState, nodeID gossip.NodeID) *gossipProtobuffer.EndpointState {
	appStates := make(map[string]*gossipProtobuffer.ApplicationState)
	for key, appState := range es.GetApplicationStates() {
		appStates[string(key)] = &gossipProtobuffer.ApplicationState{
			Key:     string(key),
			Value:   appState.Value,
			Version: appState.Version,
		}
	}

	return &gossipProtobuffer.EndpointState{
		NodeId:            string(nodeID),
		Generation:        es.HeartbeatState.Generation,
		Version:           es.HeartbeatState.Version,
		ApplicationStates: appStates,
		UpdateTimestamp:   es.GetUpdateTimestamp(),
	}
}

// GossipServiceServer implements the GossipService gRPC server
type GossipServiceServer struct {
	gossipProtobuffer.UnimplementedGossipServiceServer
	nodeID           string
	clusterID        string
	handler          GossipDigestHandler
	onPeerDiscovered PeerDiscoveryCallback
}

// GossipDigestSyn handles incoming SYN messages (Phase 1 of 3-phase gossip)
func (s *GossipServiceServer) GossipDigestSyn(ctx context.Context, req *gossipProtobuffer.GossipDigestSynMsg) (*gossipProtobuffer.GossipDigestAck, error) {
	if req.SenderAddress != "" {
		logger.Printf("GossipServiceServer[%s]: Received SYN from %s (cluster %s) with %d digests",
			s.nodeID, req.SenderAddress, req.ClusterId, len(req.Digests))
	} else {
		logger.Printf("GossipServiceServer[%s]: Received SYN from cluster %s with %d digests",
			s.nodeID, req.ClusterId, len(req.Digests))
	}

	// Discover peer from sender address
	if s.onPeerDiscovered != nil && req.SenderAddress != "" {
		if err := s.onPeerDiscovered(req.SenderAddress); err != nil {
			logger.Printf("Failed to add peer %s: %v", req.SenderAddress, err)
		}
	}

	// Log received digests for debugging
	for _, digest := range req.Digests {
		logger.Printf("  Digest: node=%s, gen=%d, maxVer=%d",
			digest.NodeId, digest.Generation, digest.MaxVersion)
	}

	// If we have a handler, use it to process the SYN
	if s.handler != nil {
		// Convert proto → gossip
		remoteDigests := make([]gossip.GossipDigest, 0, len(req.Digests))
		for _, pd := range req.Digests {
			remoteDigests = append(remoteDigests, protoDigestToGossip(pd))
		}

		// Call handler with gossip types
		endpointStateMap, requestDigests, err := s.handler.HandleGossipSyn(req.ClusterId, remoteDigests)
		if err != nil {
			return nil, err
		}

		// Convert gossip → proto for response
		protoEndpointStates := make([]*gossipProtobuffer.EndpointState, 0, len(endpointStateMap))
		for nodeID, es := range endpointStateMap {
			protoEndpointStates = append(protoEndpointStates, endpointStateToProto(es, nodeID))
		}

		protoRequestDigests := make([]*gossipProtobuffer.GossipDigest, 0, len(requestDigests))
		for _, rd := range requestDigests {
			protoRequestDigests = append(protoRequestDigests, gossipDigestToProto(rd))
		}

		return &gossipProtobuffer.GossipDigestAck{
			EndpointStates: protoEndpointStates,
			RequestDigests: protoRequestDigests,
		}, nil
	}

	// Default: return empty ACK
	return &gossipProtobuffer.GossipDigestAck{
		EndpointStates: []*gossipProtobuffer.EndpointState{},
		RequestDigests: []*gossipProtobuffer.GossipDigest{},
	}, nil
}

// Heartbeat handles heartbeat requests
func (s *HeartbeatServiceServer) Heartbeat(ctx context.Context, req *gossipProtobuffer.HeartbeatRequest) (*gossipProtobuffer.HeartbeatResponse, error) {
	logger.Printf("HeartbeatServiceServer: Heartbeat received from %s", req.NodeId)
	// Convert proto → gossip types and call handler
	localNodeID, _, _, err := s.handler.HandleHeartbeat(
		req.NodeId,
		req.Timestamp,
		0, // Version not in proto request, using 0 for now
	)

	if err != nil {
		return nil, err
	}

	// Convert gossip → proto types
	// Note: proto response only has node_id and timestamp, so we use Generation as timestamp
	return &gossipProtobuffer.HeartbeatResponse{
		NodeId:    localNodeID,
		Timestamp: time.Now().Unix(), // Using Generation as timestamp in response
	}, nil
}
