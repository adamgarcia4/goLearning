package gossip

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

/**
My Application State needs to answer 3 questions:
1. Who are the nodes? (membership list)
2. Are they alive? (Liveness)
3. How do I contact them? (Addressability)

Discovery: GossipState.StateByNode
Liveness: GossipState.StateByNode.Heartbeat.Generation
Addressability: GossipState.StateByNode.AppStates[AppHeartbeat].Value
*/

type GossipState struct {
	nodeID            NodeID
	mu                sync.RWMutex
	heartbeatInterval time.Duration
	myHeartbeatState  HeartbeatState

	// StateByNode map[NodeID]*EndpointState
}

// HeartbeatSender is a function that sends a heartbeat and returns the response node ID and timestamp
type HeartbeatSender func(heartbeatState HeartbeatState) (string, int64, error)

func (g *GossipState) SendHeartbeat(sendHeartbeat HeartbeatSender) (string, int64, error) {
	g.mu.Lock()
	g.myHeartbeatState.Version++
	heartbeatState := g.myHeartbeatState
	g.mu.Unlock()
	return sendHeartbeat(heartbeatState)
}

func (g *GossipState) InitializeHeartbeatSending(ctx context.Context, sendHeartbeat HeartbeatSender) {
	ticker := time.NewTicker(g.heartbeatInterval)
	defer ticker.Stop()
	log.Printf("Node %s: Starting to send heartbeats every %v\n", g.nodeID, g.heartbeatInterval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _, err := g.SendHeartbeat(sendHeartbeat)
			if err != nil {
				log.Printf("Node %s: Failed to send heartbeat: %v\n", g.nodeID, err)
			}
		}
	}
}

func (g *GossipState) LocalHeartbeat() HeartbeatState {
	g.mu.RLock()
	hb := g.myHeartbeatState
	g.mu.RUnlock()
	return hb
}

func (g *GossipState) Start(ctx context.Context, sendHeartbeat HeartbeatSender) {
	go g.InitializeHeartbeatSending(ctx, sendHeartbeat)
}

func NewGossipState(nodeID NodeID, interval time.Duration) (*GossipState, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("interval must be greater than 0")
	}

	if nodeID == "" {
		return nil, fmt.Errorf("nodeID must be set")
	}

	return &GossipState{
		nodeID:            nodeID,
		heartbeatInterval: interval,
		myHeartbeatState: HeartbeatState{
			NodeID:     nodeID,
			Generation: time.Now().Unix(),
			Version:    0,
		},
	}, nil
}
