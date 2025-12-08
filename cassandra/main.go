package main

import (
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	_ "github.com/adamgarcia4/goLearning/cassandra/api/gossip/v1" // Import to register proto file descriptors for reflection
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

	// Register gossip services
	gossip.RegisterServices(grpcServer, args.nodeID)

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
			if err := gossip.StartClient(args.nodeID, args.targetServer, 5*time.Second); err != nil {
				log.Fatalf("client error: %v", err)
			}
		}()
		log.Printf("Client mode enabled: sending heartbeats to %s every 5 seconds\n", args.targetServer)
	}

	handleGrpcServer(args)
}
