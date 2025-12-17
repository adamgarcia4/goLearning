package transport

import (
	"context"
	"time"

	gossipProtobuffer "github.com/adamgarcia4/goLearning/cassandra/api/gossip/v1" // Import to register proto file descriptors for reflection
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
	// HandleGossipSyn processes an incoming SYN message and returns an ACK
	// For now, this is a placeholder that just logs and returns empty ACK
	HandleGossipSyn(clusterID string, digests []*gossipProtobuffer.GossipDigest) (*gossipProtobuffer.GossipDigestAck, error)
}

// PeerDiscoveryCallback is called when a new peer is discovered through gossip
type PeerDiscoveryCallback func(peerAddr string) error

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
		return s.handler.HandleGossipSyn(req.ClusterId, req.Digests)
	}

	// Default: return empty ACK (Phase 2 logic will be implemented later)
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
