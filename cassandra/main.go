package main

import (
	"log"
	"time"

	"github.com/adamgarcia4/goLearning/cassandra/gossip"
)

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
