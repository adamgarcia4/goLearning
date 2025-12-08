package node

import "time"

func (n *Node) startGossipLoop() {
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ticker.C:
			n.Gossip.TickHeartbeat()
		case <-n.stop:
			return
		}
	}
}
