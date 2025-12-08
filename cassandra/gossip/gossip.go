package gossip

import (
	"log"
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
	heartbeatInterval time.Duration
	myHeartbeatState  HeartbeatState

	// StateByNode map[NodeID]*EndpointState
}

// HeartbeatSender is a function that sends a heartbeat and returns the response node ID and timestamp
type HeartbeatSender func(heartbeatState HeartbeatState) (string, int64, error)

func (g *GossipState) SendHeartbeat(sendHeartbeat HeartbeatSender) (err error) {
	g.myHeartbeatState.Version++

	responseNodeID, responseTimestamp, err := sendHeartbeat(g.myHeartbeatState)
	if err != nil {
		return err
	}

	log.Printf("Node %s: Sent heartbeat, received response from %s (timestamp: %d)\n",
		g.myHeartbeatState.NodeID, responseNodeID, responseTimestamp)

	return nil
}

func (g *GossipState) InitializeHeartbeatSending(sendHeartbeat HeartbeatSender) {
	ticker := time.NewTicker(g.heartbeatInterval)
	defer ticker.Stop()
	log.Printf("Node %s: Starting to send heartbeats every %v\n", g.myHeartbeatState.NodeID, g.heartbeatInterval)

	for range ticker.C {
		err := g.SendHeartbeat(sendHeartbeat)
		if err != nil {
			log.Printf("Node %s: Failed to send heartbeat: %v\n", g.myHeartbeatState.NodeID, err)
			continue
		}
	}
}

// StartClient starts a ticker that sends heartbeats using the provided sender function
func StartClient(nodeID NodeID, interval time.Duration, sendHeartbeat HeartbeatSender) (*GossipState, error) {
	gossipState := GossipState{
		heartbeatInterval: interval,
		myHeartbeatState: HeartbeatState{
			NodeID:     nodeID,
			Generation: time.Now().Unix(),
			Version:    0,
		},
	}

	go gossipState.InitializeHeartbeatSending(sendHeartbeat)

	return &gossipState, nil
}
