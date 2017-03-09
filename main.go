package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	host "github.com/libp2p/go-libp2p-host"
	nat "github.com/libp2p/go-libp2p-nat"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	swarm "github.com/libp2p/go-libp2p-swarm"
	bhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	testutil "github.com/libp2p/go-testutil"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"

	natinfo "github.com/whyrusleeping/natest/natinfo"
)

type NatCheck struct {
	Error      error
	MappedAddr ma.Multiaddr
}

type Results struct {
	Nat                 NatCheck
	HavePublicIP        bool
	Response            natinfo.NATResponse
	TcpReuseportWorking bool
}

type ServerInfo struct {
	A pstore.PeerInfo
	B pstore.PeerInfo

	SeenAddr ma.Multiaddr
}

type pinfo struct {
	ID    string
	Addrs []string
}

func (p pinfo) toPeerInfo() (pstore.PeerInfo, error) {
	pid, err := peer.IDB58Decode(p.ID)
	if err != nil {
		return pstore.PeerInfo{}, err
	}

	out := pstore.PeerInfo{ID: pid}
	for _, a := range p.Addrs {
		addr, err := ma.NewMultiaddr(a)
		if err != nil {
			return pstore.PeerInfo{}, err
		}

		out.Addrs = append(out.Addrs, addr)
	}
	return out, nil
}

func getServerInfo(server string) (*ServerInfo, error) {
	resp, err := http.Get(server + "/peerinfo")
	if err != nil {
		return nil, fmt.Errorf("could not contact natest server: %s", err)
	}

	defer resp.Body.Close()

	var sresp struct {
		A        pinfo
		B        pinfo
		SeenAddr string
	}
	err = json.NewDecoder(resp.Body).Decode(&sresp)
	if err != nil {
		return nil, err
	}

	pia, err := sresp.A.toPeerInfo()
	if err != nil {
		return nil, err
	}

	pib, err := sresp.B.toPeerInfo()
	if err != nil {
		return nil, err
	}

	naddr, err := net.ResolveTCPAddr("tcp", sresp.SeenAddr)
	if err != nil {
		return nil, err
	}
	maddr, err := manet.FromNetAddr(naddr)
	if err != nil {
		return nil, err
	}

	return &ServerInfo{
		A:        pia,
		B:        pib,
		SeenAddr: maddr,
	}, nil
}

func tryToMakeNatMapping(addr ma.Multiaddr) (ma.Multiaddr, error) {
	onat := nat.DiscoverNAT()
	mapping, err := onat.NewMapping(addr)
	if err != nil {
		return nil, err
	}

	extaddr, err := mapping.ExternalAddr()
	if err != nil {
		return nil, err
	}
	return extaddr, nil
}

func checkIfIpInList(addrs []ma.Multiaddr, check ma.Multiaddr) bool {
	var proto int
	s, err := check.ValueForProtocol(ma.P_IP4)
	if err == nil {
		proto = ma.P_IP4
	} else {
		s, err = check.ValueForProtocol(ma.P_IP6)
		if err != nil {
			fmt.Println("check addr didnt have any ip protocols")
		}
		proto = ma.P_IP6
	}

	for _, a := range addrs {
		cs, err := a.ValueForProtocol(proto)
		if err != nil {
			return false
		}
		if s == cs {
			return true
		}
	}
	return false
}

func main() {
	defaultServer := "http://mars.i.ipfs.team:7777"
	listenF := flag.Int("l", 0, "wait for incoming connections")
	natestserver := flag.String("server", defaultServer, "url of natest server")
	noNat := flag.Bool("nonat", false, "don't use nat lib")
	flag.Parse()

	listenaddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *listenF)

	sresp, err := getServerInfo(*natestserver)
	if err != nil {
		log.Fatal(err)
	}

	// first host dials out and makes the initial request
	ha, err := makeDummyHost("/ip4/127.0.0.1/tcp/0")
	if err != nil {
		log.Fatal(err)
	}

	// second host gets dialed to from the natest server
	hb, err := makeDummyHost(listenaddr)
	if err != nil {
		log.Fatal(err)
	}

	// get addrs for listener host
	myaddrs, err := hb.Network().InterfaceListenAddresses()
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(myaddrs)
	pubIpAddr := checkIfIpInList(myaddrs, sresp.SeenAddr)
	fmt.Println("Public IP? ", pubIpAddr)

	var naterror error
	var extaddr ma.Multiaddr
	if !*noNat {
		nataddr, err := tryToMakeNatMapping(myaddrs[0])
		if err != nil {
			naterror = err
			fmt.Println("Creation of NAT Traversal mapping failed:", err)
		}

		extaddr = nataddr
	}
	_ = naterror

	err = hb.Connect(context.Background(), sresp.B)
	if err != nil {
		log.Fatalln(err)
	}

	err = ha.Connect(context.Background(), sresp.A)
	if err != nil {
		log.Fatalln(err)
	}

	var req natinfo.NATRequest
	if extaddr != nil {
		req.PortMapped = extaddr.String()
	}
	req.ListenAddr = myaddrs[0].String()
	req.PeerID = hb.ID().Pretty()

	resp, err := makeReq(ha, &req, sresp.A.ID)
	if err != nil {
		log.Fatalln(err)
	}

	out, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(string(out))

	out, err = json.MarshalIndent(resp, "", "  ")
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(string(out))

	if resp.ConnectBackAddr == req.PortMapped && req.PortMapped != "" {
		fmt.Println("your routers upnp/NAT-PMP port mapping works!")
	}
}

func pinfoFromString(target string) (*pstore.PeerInfo, error) {
	if target == "" {
		return nil, fmt.Errorf("please specify target")
	}

	ipfsaddr, err := ma.NewMultiaddr(target)
	if err != nil {
		return nil, err
	}

	pid, err := ipfsaddr.ValueForProtocol(ma.P_IPFS)
	if err != nil {
		return nil, err
	}

	peerid, err := peer.IDB58Decode(pid)
	if err != nil {
		return nil, err
	}

	tptaddr := strings.Split(ipfsaddr.String(), "/ipfs/")[0]
	tptmaddr, err := ma.NewMultiaddr(tptaddr)
	if err != nil {
		return nil, err
	}

	return &pstore.PeerInfo{
		ID:    peerid,
		Addrs: []ma.Multiaddr{tptmaddr},
	}, nil
}

// create a 'Host' with a random peer to listen on the given address
func makeDummyHost(listen string) (host.Host, error) {
	addr, err := ma.NewMultiaddr(listen)
	if err != nil {
		return nil, err
	}

	ps := pstore.NewPeerstore()
	var pid peer.ID

	ident, err := testutil.RandIdentity()
	if err != nil {
		return nil, err
	}

	ps.AddPrivKey(ident.ID(), ident.PrivateKey())
	ps.AddPubKey(ident.ID(), ident.PublicKey())
	pid = ident.ID()

	ctx := context.Background()

	// create a new swarm to be used by the service host
	netw, err := swarm.NewNetwork(ctx, []ma.Multiaddr{addr}, pid, ps, nil)
	if err != nil {
		return nil, err
	}

	return bhost.New(netw), nil
}

func makeReq(h host.Host, req *natinfo.NATRequest, peerid peer.ID) (*natinfo.NATResponse, error) {
	s, err := h.NewStream(context.Background(), peerid, "/nattest/1.0.0")
	if err != nil {
		log.Fatalln(err)
	}

	err = json.NewEncoder(s).Encode(&req)
	if err != nil {
		return nil, err
	}

	var resp natinfo.NATResponse
	err = json.NewDecoder(s).Decode(&resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}
