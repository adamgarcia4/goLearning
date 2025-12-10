package gossip

/*
*
NodeID:

	This must never change during the node's lifetime
	Remains unique cluster-wide
	Survives node restarts and re-joins
	Allows mapping to network endpoints
	Examples: uuid, {host}:{port}

Generation:

	This matches cassandra's "Generation" field
	The node's start time in unix seconds
	Used as a monotonically increasing incarnation number
	If a node restarts, its generation will be greater than any prior value.
	Thus: Restart = new generation

	Why do we need this?
	Image NodeA crashes, gossip hasn't converged, then NodeA comes back.
	Other nodes might still think oldA is DOWN/SUSPECT.
	To Override stale gossip, NodeA must present a strictly newer generation.

Version:

	This is a counter that increments on each heartbeat tick.
	Cassandra increments it every gossip interval.
	This lets other nodes see:
		1. Is this node alive?
		2. Is it sending fresh info?
		3. Is this newer info than what I have?
	Every gossip interval, this is incremented.

	When receiving a heartbeat:
		If generation is larger, this overrides all old state.
		If generation is same but version is larger, this is a newer heartbeat.
		If Version hasn't changed for X seconds -> Suspicion / marking node as failed
*/

type NodeID string

type AppStateKey string

const (
	AppStatus    AppStateKey = "STATUS"
	AppHeartbeat AppStateKey = "ADDR"
	// TODO: Add more app state keys here
)

type AppState struct {
	Value   string
	Version int64
}
