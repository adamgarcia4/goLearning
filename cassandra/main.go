package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/adamgarcia4/goLearning/cassandra/gossip"
	"github.com/adamgarcia4/goLearning/cassandra/gossip/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Config struct {
	address      string
	port         string
	nodeID       string
	clientMode   bool
	targetServer string
}

func getCliArgs() (*Config, error) {
	address := flag.String("address", "127.0.0.1", "address")
	port := flag.String("port", "50051", "port")
	nodeID := flag.String("node-id", "node-1", "node identifier")
	clientMode := flag.Bool("client", false, "run as client (send heartbeats)")
	targetServer := flag.String("target", "127.0.0.1:50051", "target server address (for client mode)")

	flag.Parse()
	return &Config{
		address:      *address,
		port:         *port,
		nodeID:       *nodeID,
		clientMode:   *clientMode,
		targetServer: *targetServer,
	}, nil
}

func listen(address string, port string) (net.Listener, error) {
	lis, err := net.Listen("tcp", address+":"+port)
	if err != nil {
		panic(err)
	}

	return lis, nil
}

func main() {
	args, err := getCliArgs()

	log.Println("args", args)
	if err != nil {
		log.Fatalf("error getting cli args: %v", err)
	}

	// Run as client if client mode is enabled
	if args.clientMode {
		if err := gossip.StartClient(args.nodeID, args.targetServer, 5*time.Second); err != nil {
			log.Fatalf("client error: %v", err)
		}
		return
	}

	// Run as server
	lis, err := listen(args.address, args.port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Create and register HeartbeatService
	heartbeatServer := gossip.NewServer(args.nodeID)
	proto.RegisterHeartbeatServiceServer(grpcServer, heartbeatServer)

	// Register reflection service for gRPC tools (grpcurl, grpcui, etc.)
	reflection.Register(grpcServer)

	fmt.Printf("gRPC server listening on %s (node-id: %s)\n", lis.Addr(), args.nodeID)

	// Start serving
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
