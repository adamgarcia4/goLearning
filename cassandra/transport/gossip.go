package transport

import (
	"context"

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

// Heartbeat handles heartbeat requests
func (s *HeartbeatServiceServer) Heartbeat(ctx context.Context, req *gossipProtobuffer.HeartbeatRequest) (*gossipProtobuffer.HeartbeatResponse, error) {
	logger.Printf("[%s] Heartbeat received from %s (generation: %d)", s.nodeID, req.NodeId, req.Timestamp)
	// Convert proto → gossip types and call handler
	localNodeID, localGeneration, _, err := s.handler.HandleHeartbeat(
		req.NodeId,
		req.Timestamp,
		0, // Version not in proto request, using 0 for now
	)

	if err != nil {
		logger.Printf("[%s] Error handling heartbeat from %s: %v", s.nodeID, req.NodeId, err)
		return nil, err
	}

	// Convert gossip → proto types
	// Note: proto response only has node_id and timestamp, so we use Generation as timestamp in response
	resp := &gossipProtobuffer.HeartbeatResponse{
		NodeId:    localNodeID,
		Timestamp: localGeneration, // Use local generation as timestamp in response
	}
	logger.Printf("[%s] Heartbeat response sent to %s (local generation: %d)", s.nodeID, req.NodeId, localGeneration)
	return resp, nil
}
