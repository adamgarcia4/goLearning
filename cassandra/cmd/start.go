package cmd

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/adamgarcia4/goLearning/cassandra/gossip"
	"github.com/adamgarcia4/goLearning/cassandra/logger"
	"github.com/adamgarcia4/goLearning/cassandra/node"
)

var (
	address      string
	port         string
	nodeID       string
	clientMode   bool
	targetServer string
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a gossip node",
	Long: `Start a gossip protocol node. The node can run in server mode (default)
or client mode (when --client flag is set).

Examples:
  # Start a node in server mode
  cassandra start --node-id=node-1 --port=50051

  # Start a node in client mode that sends heartbeats to another node
  cassandra start --node-id=node-2 --port=50052 --client --target=127.0.0.1:50051`,
	Run: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)

	// Server flags
	startCmd.Flags().StringVarP(&address, "address", "a", "127.0.0.1", "Address to bind the server to")
	startCmd.Flags().StringVarP(&port, "port", "p", "50051", "Port to bind the server to")
	startCmd.Flags().StringVarP(&nodeID, "node-id", "n", "node-1", "Unique node identifier")

	// Client flags
	startCmd.Flags().BoolVarP(&clientMode, "client", "c", false, "Run in client mode (send heartbeats)")
	startCmd.Flags().StringVarP(&targetServer, "target", "t", "127.0.0.1:50051", "Target server address (required in client mode)")
}

func runStart(cmd *cobra.Command, args []string) {
	// Initialize logger for non-interactive mode (write to stdout)
	logger.Init("", true) // No prefix, write to stdout
	
	// Create node configuration with defaults
	config := node.DefaultConfig(gossip.NodeID(nodeID))
	
	// Override with CLI flags
	config.Address = address
	config.Port = port
	config.ClientMode = clientMode
	config.TargetServer = targetServer

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

	log.Println("Shutting down...")
	if err := n.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
}
