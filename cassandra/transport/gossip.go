package transport

import (
	"context"
	"time"

	gossipProtobuffer "github.com/adamgarcia4/goLearning/cassandra/api/gossip/v1" // Import to register proto file descriptors for reflection
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
