package gossip

type NodeID string

type HeartbeatState struct {
	Generation int64 // node start time (unix seconds)
	Version    int64 // incremented on each heartbeat
}

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

type EndpointState struct {
	Heartbeat HeartbeatState
	AppStates map[AppStateKey]AppState
}
