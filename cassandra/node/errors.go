package node

import "errors"

var (
	ErrNodeIDRequired        = errors.New("node ID is required")
	ErrPortRequired          = errors.New("port is required")
	ErrAddressRequired       = errors.New("address is required")
	ErrInvalidHeartbeatInterval = errors.New("heartbeat interval must be greater than 0")
	ErrTargetServerRequired = errors.New("target server is required when in client mode")
)

