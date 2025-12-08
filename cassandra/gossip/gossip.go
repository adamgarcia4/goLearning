package gossip

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	pbproto "github.com/adamgarcia4/goLearning/cassandra/api/gossip/v1"
)

/**
My Application State needs to answer 3 questions:
1. Who are the nodes? (membership list)
2. Are they alive? (Liveness)
3. How do I contact them? (Addressability)

Discovery: GossipState.StateByNode
Liveness: GossipState.StateByNode.Heartbeat.Generation
Addressability: GossipState.StateByNode.AppStates[AppHeartbeat].Value
*/

type Server struct {
	pbproto.UnimplementedHeartbeatServiceServer
	nodeID string
}

// NewServer creates a new HeartbeatService server
func NewServer(nodeID string) *Server {
	return &Server{
		nodeID: nodeID,
	}
}

// Heartbeat handles heartbeat requests
func (s *Server) Heartbeat(ctx context.Context, req *pbproto.HeartbeatRequest) (*pbproto.HeartbeatResponse, error) {
	return &pbproto.HeartbeatResponse{
		NodeId:    s.nodeID,
		Timestamp: time.Now().Unix(),
	}, nil
}

type GossipState struct {
	StateByNode map[NodeID]*EndpointState
}

// StartClient starts a client that sends heartbeats to the target server
func StartClient(nodeID, targetAddress string, interval time.Duration) error {
	// Connect to the target server
	conn, err := grpc.NewClient(targetAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := pbproto.NewHeartbeatServiceClient(conn)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Node %s: Starting to send heartbeats to %s every %v\n", nodeID, targetAddress, interval)

	ctx := context.Background()
	for range ticker.C {
		req := &pbproto.HeartbeatRequest{
			NodeId:    nodeID,
			Timestamp: time.Now().Unix(),
		}

		resp, err := client.Heartbeat(ctx, req)
		if err != nil {
			log.Printf("Node %s: Failed to send heartbeat: %v\n", nodeID, err)
			continue
		}

		log.Printf("Node %s: Sent heartbeat to %s, received response from %s (timestamp: %d)\n",
			nodeID, targetAddress, resp.NodeId, resp.Timestamp)
	}
	// Unreachable, but required for function signature
	return nil
}

// StartServer starts a gRPC server with the HeartbeatService
func StartServer(nodeID, address, port string) error {
	lis, err := net.Listen("tcp", address+":"+port)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}
	defer lis.Close()

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Create and register HeartbeatService
	heartbeatServer := NewServer(nodeID)
	pbproto.RegisterHeartbeatServiceServer(grpcServer, heartbeatServer)

	// Register reflection service for gRPC tools (grpcurl, grpcui, etc.)
	reflection.Register(grpcServer)

	log.Printf("gRPC server listening on %s (node-id: %s)\n", lis.Addr(), nodeID)

	// Start serving
	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %v", err)
	}
	// Unreachable, but required for function signature
	return nil
}
