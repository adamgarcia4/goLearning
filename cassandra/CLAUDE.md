# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Go implementation of Cassandra's gossip protocol for distributed node discovery and failure detection. The project implements the 3-phase gossip exchange (SYN/ACK/ACK2) pattern from the Cassandra paper.

## Common Commands

```bash
# Build
go build -o cassandra .

# Run tests
go test ./...

# Run a single test
go test ./gossip -run TestHeartbeatState

# Start a node
go run . start --node-id=node-1 --port=50051

# Start interactive TUI mode (manages multiple nodes)
go run . interactive

# Regenerate protobuf code (requires protoc installed)
task proto

# Kill nodes on ports 50051/50052
task kill
```

## Architecture

### Core Packages

- **`cmd/`** - CLI commands using Cobra. `start` runs a single node; `interactive` provides a Bubbletea TUI for managing multiple nodes.

- **`gossip/`** - Core gossip protocol implementation:
  - `GossipState` - Central coordinator maintaining `StateByNode` map
  - `EndpointState` - Per-node state containing heartbeat + application states
  - `HeartbeatState` - Generation (node start time) + Version (incrementing counter)
  - Thread-safe with RWMutex protection on state access

- **`transport/`** - gRPC transport layer implementing `HeartbeatServiceServer`. Handles network communication and graceful shutdown.

- **`node/`** - Node lifecycle management. `Manager` coordinates multiple nodes in interactive mode with auto-port assignment starting at 50051.

- **`logger/`** - Multi-output logger with in-memory circular buffer (1000 entries) for TUI log display.

- **`api/gossip/v1/`** - Protobuf definitions and generated gRPC code. Two services: `HeartbeatService` for simple heartbeats, `GossipService` for 3-phase exchange.

### Gossip Protocol Flow

1. **SYN**: Node sends digests (node ID + generation + version) of all known endpoints
2. **ACK**: Receiver compares digests, responds with states the sender needs
3. **ACK2**: Sender sends any states the receiver requested

### Key Patterns

- Generation/version comparison detects node restarts (generation changes) vs normal updates (version increments)
- `isAlive` flag with timestamp tracks node liveness
- All state mutations go through `GossipState` methods for thread safety

## Proto Generation

Proto files are in `api/gossip/v1/`. After modifying `.proto` files:

```bash
protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  api/gossip/v1/heartbeat.proto
```

Or use `task proto` to regenerate all proto files.
