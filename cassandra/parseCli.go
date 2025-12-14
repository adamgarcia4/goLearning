package main

import "flag"

type NodeID string

type Config struct {
	address      string
	port         string
	nodeID       NodeID
	targetServer string
}

func getCliArgs() (*Config, error) {
	address := flag.String("address", "127.0.0.1", "address")
	port := flag.String("port", "50051", "port")
	nodeID := flag.String("node-id", "node-1", "node identifier")
	targetServer := flag.String("target", "127.0.0.1:50051", "target server address (for client mode)")

	flag.Parse()
	return &Config{
		address:      *address,
		port:         *port,
		nodeID:       NodeID(*nodeID),
		targetServer: *targetServer,
	}, nil
}
