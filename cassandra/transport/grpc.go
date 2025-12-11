package transport

import (
	"fmt"
	"net"
	"strings"
	"sync"

	gossipProtobuffer "github.com/adamgarcia4/goLearning/cassandra/api/gossip/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type GRPC struct {
	addr          string
	srv           *grpc.Server
	lis           net.Listener
	nodeID        string
	gossipHandler GossipHandler
	// logger *log.Logger
	serveErrCh chan error // Channel to receive Serve() errors (for monitoring)
	stopOnce   sync.Once  // Ensures Stop() is idempotent and thread-safe
	stopErr    error      // Captured error from lis.Close()
}

func (g *GRPC) setupTcp() (net.Listener, error) {
	lis, err := net.Listen("tcp", g.addr)

	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	return lis, nil
}

func (g *GRPC) setupServices() error {
	if g.gossipHandler == nil {
		return fmt.Errorf("gossip handler must be set")
	}

	heartbeatServer := &HeartbeatServiceServer{
		handler: g.gossipHandler,
		nodeID:  g.nodeID,
	}
	gossipProtobuffer.RegisterHeartbeatServiceServer(g.srv, heartbeatServer)
	return nil
}

// Start performs binding synchronously and returns an error immediately if binding fails.
// If binding succeeds, it spawns Serve in a goroutine and returns nil.
// The caller can check the return value to know if binding succeeded.
func (g *GRPC) Start() error {
	// Perform binding synchronously - this will return an error immediately if binding fails
	lis, err := g.setupTcp()
	if err != nil {
		return fmt.Errorf("failed to setup TCP: %w", err)
	}
	g.lis = lis

	// Register services
	if err := g.setupServices(); err != nil {
		return fmt.Errorf("failed to setup services: %w", err)
	}

	// Register reflection service for gRPC tools (grpcurl, grpcui, etc.)
	reflection.Register(g.srv)

	// Binding succeeded - now spawn Serve in a goroutine
	// The caller has already been notified of success via the nil return value
	go func() {
		err := g.srv.Serve(g.lis)
		if err != nil && g.serveErrCh != nil {
			// Send serve errors to channel (non-blocking)
			select {
			case g.serveErrCh <- err:
			default:
			}
		}
	}()

	return nil
}

// Stop gracefully stops the gRPC server.
// It is idempotent and thread-safe, and returns any error from closing the listener.
func (g *GRPC) Stop() error {
	g.stopOnce.Do(func() {
		// Stop the gRPC server gracefully (this will unblock Serve())
		if g.srv != nil {
			g.srv.GracefulStop()
		}
		// Close the listener and capture any error
		if g.lis != nil {
			g.stopErr = g.lis.Close()
		}
	})
	return g.stopErr
}

// ServeErrors returns a receive-only channel that receives errors from the gRPC server's Serve() method.
// Callers should read from this channel to detect post-bind Serve() failures that occur after Start() returns successfully.
// The channel is buffered and initialized when the server is created, so it's safe to call this method
// even before Start() is called. Errors are sent non-blocking, so if the channel is full, subsequent errors may be dropped.
func (g *GRPC) ServeErrors() <-chan error {
	return g.serveErrCh
}

func NewGRPC(addr string, nodeID string, gossipHandler GossipHandler) (*GRPC, error) {
	if addr == "" || !strings.Contains(addr, ":") {
		return nil, fmt.Errorf("invalid address: %s", addr)
	}

	if nodeID == "" {
		return nil, fmt.Errorf("nodeID must be provided")
	}

	if gossipHandler == nil {
		return nil, fmt.Errorf("gossip handler must be provided")
	}

	return &GRPC{
		addr:          addr,
		srv:           grpc.NewServer(),
		nodeID:        nodeID,
		gossipHandler: gossipHandler,
		serveErrCh:    make(chan error, 1), // Buffered channel for serve errors
	}, nil
}
