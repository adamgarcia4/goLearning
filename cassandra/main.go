package main

import (
	"flag"
	"log"
	"time"

	"github.com/adamgarcia4/goLearning/cassandra/gossip"
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

	// Start gRPC server
	if err := gossip.StartServer(args.nodeID, args.address, args.port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
