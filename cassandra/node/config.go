package node

import (
	"time"

	"github.com/adamgarcia4/goLearning/cassandra/gossip"
)

// Default configuration constants
const (
	DefaultAddress   = "127.0.0.1"
	DefaultPort      = "50051"
	DefaultNodeID    = "node-1"
	DefaultClusterID = "default-cluster"
)

// Config holds the configuration for a node
type Config struct {
	// Node identification
	NodeID    gossip.NodeID
	ClusterID string

	// Server configuration
	Address string
	Port    string

	// Peer configuration
	Seeds []string // List of seed node addresses (e.g., ["127.0.0.1:50051", "127.0.0.1:50052"])

	// Gossip configuration
	HeartbeatInterval time.Duration
	ManualHeartbeat   bool // If true, gossip rounds are triggered manually instead of on a timer
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig(nodeID gossip.NodeID) *Config {
	return &Config{
		NodeID:            nodeID,
		ClusterID:         DefaultClusterID,
		Address:           DefaultAddress,
		Port:              DefaultPort,
		Seeds:             []string{},
		HeartbeatInterval: 2 * time.Second,
	}
}

// Validate checks if the config is valid
func (c *Config) Validate() error {
	if c.NodeID == "" {
		return ErrNodeIDRequired
	}
	if c.ClusterID == "" {
		return ErrClusterIDRequired
	}
	if c.Address == "" {
		return ErrAddressRequired
	}
	if c.Port == "" {
		return ErrPortRequired
	}
	if c.HeartbeatInterval <= 0 {
		return ErrInvalidHeartbeatInterval
	}
	return nil
}

// GetAddress returns the full address (address:port)
func (c *Config) GetAddress() string {
	return c.Address + ":" + c.Port
}
