package node

import (
	"time"

	"github.com/adamgarcia4/goLearning/cassandra/gossip"
)

// Default configuration constants
const (
	DefaultAddress = "127.0.0.1"
	DefaultPort    = "50051"
	DefaultNodeID  = "node-1"
	DefaultTarget  = "127.0.0.1:50051"
)

// Config holds the configuration for a node
type Config struct {
	// Node identification
	NodeID gossip.NodeID

	// Server configuration
	Address string
	Port    string

	// Client configuration (optional)
	TargetServer string

	// Gossip configuration
	HeartbeatInterval time.Duration
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig(nodeID gossip.NodeID) *Config {
	return &Config{
		NodeID:            nodeID,
		Address:           DefaultAddress,
		Port:              DefaultPort,
		TargetServer:      DefaultTarget,
		HeartbeatInterval: 2 * time.Second,
	}
}

// Validate checks if the config is valid
func (c *Config) Validate() error {
	if c.NodeID == "" {
		return ErrNodeIDRequired
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
