package main

import (
	"context"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	_ "github.com/adamgarcia4/goLearning/cassandra/api/gossip/v1" // Import to register proto file descriptors for reflection
	pbproto "github.com/adamgarcia4/goLearning/cassandra/api/gossip/v1"
	"github.com/adamgarcia4/goLearning/cassandra/gossip"
)

func handleGrpcServer(args *Config) {
	lis, err := net.Listen("tcp", args.address+":"+args.port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Register gossip services (TODO: move to transport layer)
	// gossip.RegisterServices(grpcServer, args.nodeID)

	// Register reflection service for gRPC tools (grpcurl, grpcui, etc.)
	reflection.Register(grpcServer)

	log.Printf("gRPC server listening on %s (node-id: %s)\n", lis.Addr(), args.address+":"+args.port)

	// Start serving
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func main() {
	args, err := getCliArgs()

	if err != nil {
		log.Fatalf("error getting cli args: %v", err)
	}

	// Start client in a goroutine if client mode is enabled
	if args.clientMode {
		go func() {
			// Create gRPC client connection
			conn, err := grpc.NewClient(args.targetServer, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("failed to connect: %v", err)
			}
			defer conn.Close()

			client := pbproto.NewHeartbeatServiceClient(conn)
			ctx := context.Background()

			// Create heartbeat sender function that uses gRPC client
			sendHeartbeat := func(nodeID string, timestamp int64) (string, int64, error) {
				req := &pbproto.HeartbeatRequest{
					NodeId:    nodeID,
					Timestamp: timestamp,
				}

				resp, err := client.Heartbeat(ctx, req)
				if err != nil {
					return "", 0, err
				}

				return resp.NodeId, resp.Timestamp, nil
			}

			if _, err := gossip.StartClient(args.nodeID, 5*time.Second, sendHeartbeat); err != nil {
				log.Fatalf("client error: %v", err)
			}
		}()
		log.Printf("Client mode enabled: sending heartbeats to %s every 5 seconds\n", args.targetServer)
	}

	handleGrpcServer(args)
}
