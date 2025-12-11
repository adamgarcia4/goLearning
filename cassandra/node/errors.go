package node

import "errors"

var (
	ErrNodeIDRequired      = errors.New("node ID is required")
	ErrPortRequired        = errors.New("port is required")
	ErrTargetServerRequired = errors.New("target server is required when in client mode")
)

