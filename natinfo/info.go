package info

type NATResponse struct {
	SeenAddr           string
	ConnectBackSuccess bool
	ConnectBackMsg     string
	ConnectBackAddr    string
}

type NATRequest struct {
	PeerID      string
	SeenGateway string
	PortMapped  string
	ListenAddr  string
}
