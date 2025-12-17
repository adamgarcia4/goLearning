# Gossip Protocol Reference

## Overview

The 3-phase gossip protocol (SYN → ACK → ACK2) efficiently synchronizes distributed node state using digest-based comparison. Instead of sending full state every time, nodes exchange compact summaries to identify what's out of sync, then only transfer the needed deltas.

---

## Protocol Flow

```
    Node A (Initiator)                           Node B (Peer)
    ==================                           =============

    State:                                       State:
    ┌──────────────────┐                        ┌──────────────────┐
    │ NodeA: G=1000,V=50│                       │ NodeA: G=900,V=30│ ← OLD
    │ NodeB: G=2000,V=30│ ← OLD                 │ NodeB: G=2000,V=45│ ← NEW
    │ NodeC: G=3000,V=10│                       │ [NodeC unknown]  │
    └──────────────────┘                        └──────────────────┘

         │  PHASE 1: SYN (Digests)
         │  Digests: NodeA(G=1000,V=50), NodeB(G=2000,V=30), NodeC(G=3000,V=10)
         ├─────────────────────────────────────────────────────────────────────►
         │                                              │
         │                                   Compares each digest:
         │                                   • NodeA: Remote newer → REQUEST
         │                                   • NodeB: Local newer → SEND
         │                                   • NodeC: Unknown → REQUEST
         │
         │  PHASE 2: ACK (States + Requests)
         │  EndpointStates: [NodeB full state]
         │  RequestDigests: [NodeA, NodeC]
         ◄─────────────────────────────────────────────────────────────────────┤
         │
         │  Initiator: Merges NodeB, gathers NodeA & NodeC
         │
         │  PHASE 3: ACK2 (Requested States)
         │  EndpointStates: [NodeA full state, NodeC full state]
         ├─────────────────────────────────────────────────────────────────────►
         │                                              │
                                                   Merges NodeA, NodeC
                   ✅ BOTH NODES NOW SYNCHRONIZED ✅
```

---

## Data Structures

### GossipDigest (~24-37 bytes per node)
Compact summary for efficient comparison:
```go
type GossipDigest struct {
    NodeID     string  // Node identifier
    Generation int64   // Node start time (detects restarts)
    MaxVersion int64   // Highest version across all states
}
```

### EndpointState (~300-800 bytes per node)
Full state including all application data:
```go
type EndpointState struct {
    NodeID            string
    Generation        int64
    Version           int64
    ApplicationStates map[string]*AppState  // STATUS, ADDR, LOAD, TOKENS, etc.
    UpdateTimestamp   int64
    IsAlive           bool
}
```

### Message Types

| Phase | Message | Contents |
|-------|---------|----------|
| 1 | GossipDigestSyn | `[]GossipDigest` - summaries of all known nodes |
| 2 | GossipDigestAck | `map[NodeID]*EndpointState` (sending) + `[]GossipDigest` (requesting) |
| 3 | GossipDigestAck2 | `map[NodeID]*EndpointState` - requested states |

---

## State Comparison Logic

```
FOR EACH remote digest:
  IF local.generation > remote.generation:
    → Send local state (we're newer - they restarted with old data)
  ELSE IF local.generation < remote.generation:
    → Request their state (they restarted more recently)
  ELSE: // Same generation
    IF local.maxVersion > remote.maxVersion:
      → Send local state (we have newer updates)
    ELSE IF local.maxVersion < remote.maxVersion:
      → Request their state (they have newer updates)
    ELSE:
      → No action (already in sync)

FOR EACH local node NOT in remote digests:
  → Send local state (they don't know about this node)
```

---

## Why 3 Phases? Bandwidth Efficiency

### Size Comparison (10-node cluster)

| Approach | Phase 1 | Phase 2 | Phase 3 | Total |
|----------|---------|---------|---------|-------|
| Naive (send all) | 6,000 B | 1,800 B | - | 7,800 B |
| 3-Phase (digests) | 370 B | 1,874 B | 1,200 B | 3,444 B |

**Savings: 56% bandwidth reduction** (grows to 85-95% in larger clusters)

### Key Insight: Most nodes are already in sync
In a healthy cluster with 1-second gossip intervals, typically only 10-20% of nodes have changed. Digests let us identify exactly which ones, avoiding redundant transfers.

### MaxVersion Optimization
Instead of sending version for each ApplicationState, the digest contains only the maximum version:
```
MaxVersion = max(HeartbeatVersion, STATUS.Version, ADDR.Version, LOAD.Version, ...)
```
This compresses 5-15 version numbers into 1, at the cost of occasionally requesting slightly more than needed.

---

## Cassandra Source Code Reference

### Key Files
- `GossipDigestAck.java` - ACK message with `Map<InetAddressAndPort, EndpointState> epStateMap`
- `GossipDigestSynVerbHandler.java` - Receives SYN, builds ACK response
- `Gossiper.java` - Core comparison logic (`examineGossiper`, `sendAll`, `getStateForVersionBiggerThan`)
- `GossipDigestAckVerbHandler.java` - Receives ACK, sends ACK2

### ACK Building Flow (Cassandra)
```java
// In GossipDigestSynVerbHandler.createNormalReply()
Map<InetAddressAndPort, EndpointState> deltaEpStateMap = new HashMap<>();
Gossiper.instance.examineGossiper(gDigestList, deltaGossipDigestList, deltaEpStateMap);
return new GossipDigestAck(deltaGossipDigestList, deltaEpStateMap);
```

### State Selection (Cassandra)
```java
// In Gossiper.sendAll() - adds full EndpointState to ACK
EndpointState state = getStateForVersionBiggerThan(endpoint, maxRemoteVersion);
if (state != null) {
    deltaEpStateMap.put(endpoint, state);  // Full state added here
}
```

---

## Implementation Checklist

### Core Functions Needed
1. `CreateDigests()` - Generate digests from current state
2. `CompareStates(remoteDigests)` - Returns (statesToSend, digestsToRequest)
3. `MergeRemoteStates(states)` - Merge received states into local state
4. `GossipRound(peer)` - Execute full 3-phase exchange

### Merge Rules
- **New node**: Accept all state
- **Higher generation**: Accept all (node restarted)
- **Lower generation**: Ignore (stale data)
- **Same generation**: Merge by version (higher wins)

### Thread Safety
- Use `RWMutex` on state map
- Create snapshots under lock, process without lock
- Minimize lock hold time during network I/O

---

## Common Pitfalls

| Pitfall | Problem | Solution |
|---------|---------|----------|
| Stale generations | Restart with old generation = can't override | Generation = node start time (always increases) |
| Lock contention | Holding locks during gossip | Snapshot under lock, release before network calls |
| Missing nodes | Not sending unknown node states | Check for local nodes not in remote digest |
| Version conflicts | Concurrent updates thrash | Version per app state key, higher always wins |

---

## Performance Characteristics

### Bandwidth per Round
- Phase 1: ~37 bytes × N nodes
- Phase 2-3: ~600 bytes × out-of-sync nodes

### Convergence Time
- O(log N) rounds for full cluster convergence
- N=10: ~3-4 seconds
- N=100: ~6-7 seconds
- N=1000: ~9-10 seconds

### CPU per Round
- O(N) digest creation/comparison
- O(K) state merging where K = out-of-sync nodes
