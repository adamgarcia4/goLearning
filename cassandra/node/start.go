
func (n *Node) Start() error {
	go n.startGRPCServer()
	go n.startHeartbeatLoop()
	go n.startGossipLoop()

	return nil
}
