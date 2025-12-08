package transport

import (
	"log"
	"net"

	"github.com/adamgarcia4/goLearning/cassandra/node"
	"google.golang.org/grpc"
)

type GRPC struct {
	addr   string
	srv    *grpc.Server
	lis    net.Listener
	logger *log.Logger
	eng    *node.GossipEngine
}

func NewGRPC(addr string, eng *node.GossipEngine, logger *log.Logger) (*GRPC, error) {
	if logger == nil {
		logger = log.Default()
	}

	lis, err := net.Listen("tcp", addr)

	if err != nil {
		return nil, err
	}

}
