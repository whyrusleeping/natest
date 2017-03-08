package info

type NATResponse struct {
	SeenAddr           string
	ConnectBackSuccess bool
	ConnectBackMsg     string
	ConnectBackAddr    string
	TriedAddrs         []string
}

type NATRequest struct {
	PeerID      string
	SeenGateway string
	PortMapped  string
	ListenAddr  string
}
