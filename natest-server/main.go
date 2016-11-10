package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	bhost "gx/ipfs/QmQfvKShQ2v7nkfCE4ygisxpcSBFvBYaorQ54SibY6PGXV/go-libp2p/p2p/host/basic"
	ma "gx/ipfs/QmUAQaWbKxGCUTuoQVvvicbQNZ9APF5pDGWyAZSe93AtKH/go-multiaddr"
	host "gx/ipfs/QmWf338UyG5DKyemvoFiomDPtkVNHLsw3GAt9XXHX5ZtsM/go-libp2p-host"
	pstore "gx/ipfs/QmXXCcQ7CLg5a81Ui9TTR35QcR4y7ZyihxwfjqaHfUVcVo/go-libp2p-peerstore"
	testutil "gx/ipfs/QmaEcA713Y54EtSsj7ZYfwXmsTfxrJ4oywr1iFt1d6LKY5/go-testutil"
	swarm "gx/ipfs/QmcjMKTqrWgMMCExEnwczefhno5fvx7FHDV63peZwDzHNF/go-libp2p-swarm"
	net "gx/ipfs/QmdysBu77i3YaagNtMAjiCJdeWWvds18ho5XEB784guQ41/go-libp2p-net"
	peer "gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"

	natinfo "github.com/whyrusleeping/natest/natinfo"
)

// create a 'Host' with a random peer to listen on the given address
func makebasicHost(listen string) (host.Host, error) {
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

	addrs, _ := netw.InterfaceListenAddresses()

	for _, a := range addrs {
		log.Printf("I am %s/ipfs/%s\n", a, pid.Pretty())
	}
	return bhost.New(netw), nil
}

func main() {
	listenF := flag.Int("l", 0, "wait for incoming connections")

	flag.Parse()

	listenaddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *listenF)

	ha, err := makebasicHost(listenaddr)
	if err != nil {
		log.Fatal(err)
	}

	// Set a stream handler on host A
	ha.SetStreamHandler("/nattest/1.0.0", func(s net.Stream) {
		defer s.Close()

		var req natinfo.NATRequest
		err := json.NewDecoder(s).Decode(&req)
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Println("got req")
		out, _ := json.MarshalIndent(req, "", "  ")
		fmt.Println(string(out))

		var resp natinfo.NATResponse
		resp.SeenAddr = s.Conn().RemoteMultiaddr().String()

		err = json.NewEncoder(s).Encode(&resp)
		if err != nil {
			fmt.Println(err)
			return
		}
	})

	select {}
}
