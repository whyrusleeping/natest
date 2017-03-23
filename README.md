# natest
A tool for testing NAT traversal.

NOTE: this is still very early and definitely does not catch all scenarios. If
it reports something different than you're expecting, please file an issue.

## Installation
```
go get github.com/whyrusleeping/natest
```

## Usage
```
> natest
your routers upnp/NAT-PMP port mapping works!
{
  "OutboundHTTP": {
    "OddPortConnection": "",
    "Port443Connection": ""
  },
  "Nat": {
    "Error": null,
    "MappedAddr": {}
  },
  "HavePublicIP": false,
  "Response": {
    "SeenAddr": "/ip4/107.3.189.76/tcp/54906",
    "ConnectBackSuccess": true,
    "ConnectBackMsg": "",
    "ConnectBackAddr": "/ip4/107.3.189.76/tcp/40823",
    "TriedAddrs": [
      "/ip4/127.0.0.1/tcp/34045",
      "/ip4/107.3.189.76/tcp/40823",
      "/ip4/107.3.189.76/tcp/34045"
    ]
  },
  "Request": {
    "PeerID": "QmYd8HNFF94TQdH2AJLgSfjTt3iVXM8hrRQ2r4boCBsZX7",
    "SeenGateway": "",
    "PortMapped": "/ip4/107.3.189.76/tcp/40823",
    "ListenAddr": "/ip4/127.0.0.1/tcp/34045"
  },
  "TcpReuseportWorking": false
}
```

