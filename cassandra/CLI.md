# Cassandra CLI Usage

The Cassandra CLI provides a simple interface to start gossip protocol nodes.

## Running

You can run the CLI in two ways:

### Option 1: Direct execution (no build required)
```bash
# From the cassandra directory
go run . start --node-id=node-1 --port=50051
```

### Option 2: Build and run
```bash
# Build the binary
go build -o cassandra .

# Then run it
./cassandra start --node-id=node-1 --port=50051
```

## Usage

### Start a Node (Server Mode)

Start a node that listens for incoming heartbeats:

```bash
# Using default values (node-id=node-1, port=50051, address=127.0.0.1)
./cassandra start

# With custom values
./cassandra start --node-id=node-1 --port=50051 --address=127.0.0.1
```

### Start a Node (Client Mode)

Start a node that sends heartbeats to another node:

```bash
./cassandra start --node-id=node-2 --port=50052 --client --target=127.0.0.1:50051
```

## Command Reference

### `start` Command

Starts a gossip protocol node.

**Flags:**
- `-a, --address string`: Address to bind the server to (default: "127.0.0.1")
- `-p, --port string`: Port to bind the server to (default: "50051")
- `-n, --node-id string`: Unique node identifier (default: "node-1")
- `-c, --client`: Run in client mode (send heartbeats)
- `-t, --target string`: Target server address (required in client mode, default: "127.0.0.1:50051")

**Examples:**

Using `go run` (no build required):
```bash
# Start node A (server)
go run . start --port=50051 --node-id=node-1

# Start node B (client, sends to node A)
go run . start --port=50052 --node-id=node-2 --client --target=127.0.0.1:50051
```

Or using the built binary:
```bash
# Start node A (server)
./cassandra start --port=50051 --node-id=node-1

# Start node B (client, sends to node A)
./cassandra start --port=50052 --node-id=node-2 --client --target=127.0.0.1:50051
```

## Comparison with Taskfile

The CLI replaces the Taskfile commands:

**Taskfile:**
```bash
task startA  # go run . --port=50051 --node-id=node-1
task startB  # go run . --port=50052 --node-id=node-2 --client --target=127.0.0.1:50051
```

**CLI:**
```bash
./cassandra start --port=50051 --node-id=node-1
./cassandra start --port=50052 --node-id=node-2 --client --target=127.0.0.1:50051
```

