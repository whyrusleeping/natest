package info

type NATResponse struct {
	SeenAddr string
}

type NATRequest struct {
	SeenGateway string
	PortMapped  string
}
