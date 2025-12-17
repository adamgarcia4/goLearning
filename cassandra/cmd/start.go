package cmd

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/adamgarcia4/goLearning/cassandra/gossip"
	"github.com/adamgarcia4/goLearning/cassandra/logger"
	"github.com/adamgarcia4/goLearning/cassandra/node"
)

var (
	address string
	port    string
	nodeID  string
	seeds   []string
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a gossip node",
	Long: `Start a gossip protocol node.

Examples:
  # Start a node
  cassandra start --node-id=node-1 --port=50051

  # Start a node with seeds (peers to gossip with)
  cassandra start --node-id=node-2 --port=50052 --seeds=127.0.0.1:50051`,
	Run: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)

	// Server flags
	startCmd.Flags().StringVarP(&address, "address", "a", node.DefaultAddress, "Address to bind the server to")
	startCmd.Flags().StringVarP(&port, "port", "p", node.DefaultPort, "Port to bind the server to")
	startCmd.Flags().StringVarP(&nodeID, "node-id", "n", node.DefaultNodeID, "Unique node identifier")

	// Gossip flags
	startCmd.Flags().StringSliceVarP(&seeds, "seeds", "s", []string{}, "Seed node addresses for gossip (comma-separated)")
}

func runStart(cmd *cobra.Command, args []string) {
	// Initialize logger for non-interactive mode (write to stdout)
	logger.Init("", true) // No prefix, write to stdout

	// Create node configuration with defaults
	config := node.DefaultConfig(gossip.NodeID(nodeID))

	// Override with CLI flags
	config.Address = address
	config.Port = port
	config.Seeds = seeds

	// Create and start the node
	n, err := node.New(config)
	if err != nil {
		log.Fatalf("failed to create node: %v", err)
	}

	if err := n.Start(); err != nil {
		log.Fatalf("failed to start node: %v", err)
	}

	// Wait for interrupt signal for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down...")
	if err := n.Stop(); err != nil {
		logger.Errorf("Error during shutdown: %v", err)
	}
}
