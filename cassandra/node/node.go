package node

import (
	"context"
	"time"

	"honnef.co/go/tools/config"
	// "github.com/adamgarcia4/goLearning/cassandra/config"
)

type GossipEngine interface {
	TickHeartbeat()
	GossipOnce(ctx context.Context)
}

type GRPCServer interface {
	Serve(ctx context.Context) error // blocks until ctx done or error
	Shutdown() error
	Addr() string
}

type Node struct {
	ID        string
	Gossip    GossipEngine
	Transport *transport.GRPC
	Config    *config.Config
}

func (n *Node) startHeartbeatLoop() {
	ticker := time.NewTicker(1 * time.Second)

	for {
		select {
		case <-ticker.C:
			n.Gossip.SendHeartbeat()
			// case <-n.stop:
			// 	return
		}
	}
}

func (n *Node) startGRPCServer() {
	n.Transport.ListenAndServe()
}
