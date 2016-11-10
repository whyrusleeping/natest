package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strings"

	nat "gx/ipfs/QmPpncQ3L4bC3rnwLBrgEomygs5RbnFejb68GgsecxbMiL/go-libp2p-nat"
	bhost "gx/ipfs/QmQfvKShQ2v7nkfCE4ygisxpcSBFvBYaorQ54SibY6PGXV/go-libp2p/p2p/host/basic"
	ma "gx/ipfs/QmUAQaWbKxGCUTuoQVvvicbQNZ9APF5pDGWyAZSe93AtKH/go-multiaddr"
	host "gx/ipfs/QmWf338UyG5DKyemvoFiomDPtkVNHLsw3GAt9XXHX5ZtsM/go-libp2p-host"
	pstore "gx/ipfs/QmXXCcQ7CLg5a81Ui9TTR35QcR4y7ZyihxwfjqaHfUVcVo/go-libp2p-peerstore"
	testutil "gx/ipfs/QmaEcA713Y54EtSsj7ZYfwXmsTfxrJ4oywr1iFt1d6LKY5/go-testutil"
	swarm "gx/ipfs/QmcjMKTqrWgMMCExEnwczefhno5fvx7FHDV63peZwDzHNF/go-libp2p-swarm"
	gateway "gx/ipfs/Qmf2fBLzCvFxs3vvZaoQyKSTv2rjApek4F1kzRxAfK6P4P/gateway"
	peer "gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"

	natinfo "github.com/whyrusleeping/natest/natinfo"
)

func main() {
	listenF := flag.Int("l", 0, "wait for incoming connections")
	target := flag.String("d", "", "target peer to dial")
	flag.Parse()

	listenaddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", *listenF)

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

	//myaddrs := hb.Addrs()
	gwayip, err := gateway.DiscoverGateway()
	if err != nil {
		log.Fatalln(err)
	}

	myaddrs, err := hb.Network().InterfaceListenAddresses()
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(myaddrs)
	onat := nat.DiscoverNAT()

	mapping, err := onat.NewMapping(myaddrs[0])
	if err != nil {
		log.Fatalln(err)
	}

	extaddr, err := mapping.ExternalAddr()
	if err != nil {
		log.Fatalln(err)
	}

	pi, err := pinfoFromString(*target)
	if err != nil {
		log.Fatalln(err)
	}

	err = ha.Connect(context.Background(), *pi)
	if err != nil {
		log.Fatalln(err)
	}

	var req natinfo.NATRequest
	req.PortMapped = extaddr.String()
	req.ListenAddr = myaddrs[0].String()
	req.PeerID = hb.ID().Pretty()
	req.SeenGateway = gwayip.String()

	resp, err := makeReq(ha, &req, pi.ID)
	if err != nil {
		log.Fatalln(err)
	}

	out, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(string(out))
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

	ident.PrivateKey()
	ps.AddPrivKey(ident.ID(), ident.PrivateKey())
	ps.AddPubKey(ident.ID(), ident.PublicKey())
	pid = ident.ID()

	ctx := context.Background()

	// create a new swarm to be used by the service host
	netw, err := swarm.NewNetwork(ctx, []ma.Multiaddr{addr}, pid, ps, nil)
	if err != nil {
		return nil, err
	}

	log.Printf("I am %s/ipfs/%s\n", addr, pid.Pretty())
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
