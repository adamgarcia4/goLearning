package node

import (
	"fmt"
	"sync"

	"github.com/adamgarcia4/goLearning/cassandra/gossip"
)

// Manager manages multiple nodes
type Manager struct {
	nodes       []*Node // maintain order with slice
	nodeMap     map[string]int // map node ID to index for quick lookup
	mu          sync.RWMutex
	portCounter int // for auto-assigning ports
	nextID      int // monotonically increasing counter for unique node IDs
}

// NewManager creates a new node manager
func NewManager() *Manager {
	return &Manager{
		nodes:       make([]*Node, 0),
		nodeMap:     make(map[string]int),
		portCounter: 50051, // start from default port
		nextID:      1,     // start node IDs at 1
	}
}

// CreateNode creates and starts a new node
func (m *Manager) CreateNode() (*Node, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find next available port
	port := m.findAvailablePort()
	
	// Generate unique node ID using monotonically increasing counter
	nodeID := gossip.NodeID(fmt.Sprintf("node-%d", m.nextID))
	m.nextID++ // increment counter for next node

	config := DefaultConfig(nodeID)
	config.Port = fmt.Sprintf("%d", port)
	config.Address = "127.0.0.1"

	node, err := New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}

	if err := node.Start(); err != nil {
		return nil, fmt.Errorf("failed to start node: %w", err)
	}

	// Add to slice and map
	nodeIDStr := string(nodeID)
	m.nodes = append(m.nodes, node)
	m.nodeMap[nodeIDStr] = len(m.nodes) - 1
	return node, nil
}

// DeleteNode stops and removes a node by its index in the list
func (m *Manager) DeleteNode(index int) error {
	m.mu.Lock()
	
	if index < 0 || index >= len(m.nodes) {
		m.mu.Unlock()
		return fmt.Errorf("invalid node index: %d", index)
	}

	node := m.nodes[index]
	nodeID := string(node.GetConfig().NodeID)
	
	// Remove from slice and map before unlocking
	m.nodes = append(m.nodes[:index], m.nodes[index+1:]...)
	delete(m.nodeMap, nodeID)
	
	// Rebuild map indices
	for i, n := range m.nodes {
		m.nodeMap[string(n.GetConfig().NodeID)] = i
	}
	
	m.mu.Unlock()
	
	// Stop node asynchronously to avoid blocking
	go func() {
		if err := node.Stop(); err != nil {
			// Log error but don't return it since we've already removed from list
			fmt.Printf("Error stopping node %s: %v\n", nodeID, err)
		}
	}()
	
	return nil
}

// GetNodes returns a list of all nodes (maintains order)
func (m *Manager) GetNodes() []*Node {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Return a copy to avoid race conditions
	nodes := make([]*Node, len(m.nodes))
	copy(nodes, m.nodes)
	return nodes
}

// findAvailablePort finds the next available port
func (m *Manager) findAvailablePort() int {
	// Simple implementation: increment port counter
	port := m.portCounter
	m.portCounter++
	return port
}

// StopAll stops all nodes
func (m *Manager) StopAll() error {
	m.mu.Lock()
	nodes := make([]*Node, len(m.nodes))
	copy(nodes, m.nodes)
	m.mu.Unlock()

	var errs []error
	for _, node := range nodes {
		if err := node.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors stopping nodes: %v", errs)
	}

	return nil
}
