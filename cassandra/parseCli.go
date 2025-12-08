package main

import "flag"

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
