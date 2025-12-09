<!-- 7f5c674f-42d3-460e-b47b-568d86d7fb9f 8fd7348a-1f16-4089-8562-faf08f5db989 -->
# Protocol-Agnostic Gossip Architecture

## Goal

Reorganize codebase so gossip package is protocol-agnostic (no gRPC knowledge). Clear separation:

- `gossip/` - Pure gossip logic (heartbeat, state merging) - NO gRPC dependencies
- `transport/` - gRPC transport layer that bridges gossip to gRPC
- `node/` - Runtime orchestrator that coordinates gossip + transport

## Architecture Principles

1. **Gossip package is protocol-agnostic**: No gRPC imports, no proto imports
2. **Gossip knows WHAT to send/merge**: Heartbeat logic, state merging logic
3. **Transport knows HOW to communicate**: gRPC client/server, proto serialization
4. **Node orchestrates**: Coordinates gossip engine + transport layer

## Package Organization

### `gossip/` Package (Protocol-Agnostic)

**Purpose**: Core gossip algorithm - heartbeat generation, state merging

**Files**:

- `gossip.go` - Main gossip engine implementation
  - `Engine` struct (manages gossip state)
  - `TickHeartbeat()` - Generate heartbeat (increment version, update timestamp)
  - `MergeState(remoteState)` - Merge remote state into local state
  - `GetLocalState()` - Return current local state
  - `GetStateByNode()` - Return full gossip state map
- `types.go` - Type definitions (NodeID, HeartbeatState, AppState, EndpointState, GossipState)
- `state.go` (optional) - State management helpers

**Interfaces/Dependencies**:

- NO gRPC imports
- NO proto imports
- Works with pure Go types only
- May define interfaces for transport layer to implement

### `transport/` Package (gRPC Layer)

**Purpose**: gRPC communication - serialization, network I/O

**Files**:

- `grpc.go` - gRPC transport implementation
  - `GRPC` struct (gRPC server/client)
  - `Serve(ctx)` - Start gRPC server, register services
  - `SendHeartbeat(target, state)` - Send heartbeat via gRPC
  - `RegisterGossipHandler(gossipEngine)` - Bridge gossip engine to gRPC handlers
- `server.go` (optional) - gRPC server setup, service registration
- `client.go` (optional) - gRPC client for sending heartbeats

**Interfaces/Dependencies**:

- Imports gRPC, proto packages
- Implements gRPC service handlers
- Converts between gossip types and proto types
- Calls gossip engine methods (TickHeartbeat, MergeState)

### `node/` Package (Orchestrator)

**Purpose**: Runtime orchestration - coordinates gossip + transport

**Files**:

- `node.go` - Node struct, Start(), Stop(), NewNode()
  - `Node` struct fields:
    - ID, Address, Port
    - Gossip engine (gossip.Engine)
    - Transport (transport.GRPC)
    - Supervisor (for lifecycle)
    - Context/cancel for shutdown
- `supervisor.go` - Goroutine & ticker management
  - Track all goroutines
  - Track all tickers
  - Graceful shutdown
- `grpc_server.go` - Start gRPC server via transport
  - Calls `transport.Serve()`
  - Handles server lifecycle
- `heartbeat.go` - Heartbeat ticker (1/sec)
  - Calls `gossip.TickHeartbeat()`
  - Triggers transport to send heartbeat
- `gossip_loop.go` - Gossip ticker (1/sec)
  - Calls `gossip.GossipOnce()` or similar
  - Triggers transport to gossip with peers

## Data Flow

```
Node (orchestrator)
  ├─> Gossip Engine (business logic)
  │   ├─> TickHeartbeat() → generates heartbeat state
  │   └─> MergeState() → merges remote state
  │
  ├─> Transport (general gRPC infrastructure)
  │   ├─> Provides gRPC server
  │   ├─> Provides client connections
  │   └─> Supports multiple services
  │
  └─> Gossip gRPC Adapter (gossip-specific gRPC layer)
      ├─> Registers gossip service on transport
      ├─> Receives gRPC request → converts to gossip types → calls gossip.MergeState()
      └─> Sends heartbeat → converts gossip types to proto → sends via gRPC client
```

**Note**: Transport is general-purpose. Gossip (and other services) register their handlers on the transport.

## Changes Required

### 1. Refactor `gossip/gossip.go`

- Remove all gRPC imports
- Remove all proto imports
- Remove `Server` struct (gRPC handler) - move to transport
- Remove `RegisterServices()` - move to transport
- Remove `StartClient()` - move to transport
- Add `Engine` struct with:
  - Local node state
  - State by node map
  - Methods: `TickHeartbeat()`, `MergeState()`, `GetLocalState()`

### 2. Create/Update `transport/grpc.go`

- Move gRPC server handler from gossip
- Implement gRPC service that calls gossip engine
- Convert between proto types and gossip types
- Handle gRPC client for sending heartbeats

### 3. Update `node/` package structure

- Complete node.go with proper struct
- Create supervisor.go for lifecycle
- Create grpc_server.go, heartbeat.go, gossip_loop.go
- Node coordinates gossip engine + transport

### 4. Update `main.go`

- Create Node with gossip engine + transport
- Call node.Start()
- Remove direct gRPC setup

## File Structure After

```
gossip/
├── gossip.go    # Engine struct, TickHeartbeat(), MergeState()
├── types.go     # NodeID, HeartbeatState, AppState, etc.
└── state.go     # (optional) State management

transport/
├── grpc.go      # GRPC struct, Serve(), SendHeartbeat()
└── server.go    # (optional) gRPC server setup

node/
├── node.go          # Node struct, Start(), Stop()
├── supervisor.go    # Goroutine & ticker management
├── grpc_server.go   # Start gRPC via transport
├── heartbeat.go     # Heartbeat ticker (1/sec)
└── gossip_loop.go   # Gossip ticker (1/sec)
```

## Key Interfaces

**Gossip Engine Interface** (what node expects):

```go
type GossipEngine interface {
    TickHeartbeat()                    // Generate heartbeat
    MergeState(remoteState GossipState) // Merge remote state
    GetLocalState() EndpointState      // Get current state
    GetStateByNode() map[NodeID]*EndpointState
}
```

**Transport Interface** (what node expects):

```go
type Transport interface {
    Serve(ctx context.Context) error
    SendHeartbeat(target string, state EndpointState) error
    Shutdown() error
}
```

### To-dos

- [ ] Refactor gossip.go: Remove StartServer(), add RegisterServices() function
- [ ] Update main.go: Create gRPC server, register services, handle listener/server lifecycle
- [ ] Verify build and test that server still works correctly