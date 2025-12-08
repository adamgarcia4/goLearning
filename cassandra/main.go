package main

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pbproto "github.com/adamgarcia4/goLearning/cassandra/api/gossip/v1"
	"github.com/adamgarcia4/goLearning/cassandra/gossip"
	"github.com/adamgarcia4/goLearning/cassandra/transport"
)

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
			sendHeartbeat := func(heartbeatState gossip.HeartbeatState) (string, int64, error) {
				req := &pbproto.HeartbeatRequest{
					NodeId:    string(heartbeatState.NodeID),
					Timestamp: heartbeatState.Generation, // Using Generation as timestamp for now
				}

				resp, err := client.Heartbeat(ctx, req)
				if err != nil {
					return "", 0, err
				}

				return resp.NodeId, resp.Timestamp, nil
			}

			gossipState, err := gossip.NewGossipState(gossip.NodeID(args.nodeID), 5*time.Second)
			if err != nil {
				log.Fatalf("failed to create gossip state: %v", err)
			}
			gossipState.Start(ctx, sendHeartbeat)

			// go func() {
			// 	for {
			// 		select {
			// 		case <-ctx.Done():
			// 			return
			// 		default:
			// 			currentHeartbeatState := gossipState.LocalHeartbeat()
			// 			log.Printf("Node %s: Current heartbeat state: %+v\n", args.nodeID, currentHeartbeatState)
			// 			time.Sleep(5 * time.Second)
			// 		}
			// 	}
			// }()
		}()
		log.Printf("Client mode enabled: sending heartbeats to %s every 5 seconds\n", args.targetServer)
	}

	grpcTransport, err := transport.NewGRPC(args.address+":"+args.port, string(args.nodeID))
	if err != nil {
		log.Fatalf("failed to create gRPC: %v", err)
	}

	log.Printf("gRPC server starting on %s (node-id: %s)\n", args.address+":"+args.port, args.nodeID)
	if err := grpcTransport.Start(); err != nil {
		log.Fatalf("failed to start gRPC server: %v", err)
	}
	// handleGrpcServer(args)

}
