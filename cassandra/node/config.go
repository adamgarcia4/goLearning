package node

import (
	"time"

	"github.com/adamgarcia4/goLearning/cassandra/gossip"
)

// Config holds the configuration for a node
type Config struct {
	// Node identification
	NodeID gossip.NodeID

	// Server configuration
	Address string
	Port    string

	// Client configuration (optional)
	ClientMode   bool
	TargetServer string

	// Gossip configuration
	HeartbeatInterval time.Duration
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig(nodeID gossip.NodeID) *Config {
	return &Config{
		NodeID:            nodeID,
		Address:           "127.0.0.1",
		Port:              "50051",
		ClientMode:        false,
		TargetServer:      "127.0.0.1:50051",
		HeartbeatInterval: 5 * time.Second,
	}
}

// Validate checks if the config is valid
func (c *Config) Validate() error {
	if c.NodeID == "" {
		return ErrNodeIDRequired
	}
	if c.Port == "" {
		return ErrPortRequired
	}
	if c.ClientMode && c.TargetServer == "" {
		return ErrTargetServerRequired
	}
	return nil
}

// GetAddress returns the full address (address:port)
func (c *Config) GetAddress() string {
	return c.Address + ":" + c.Port
}

