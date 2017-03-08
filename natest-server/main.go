package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	bhost "gx/ipfs/QmQfvKShQ2v7nkfCE4ygisxpcSBFvBYaorQ54SibY6PGXV/go-libp2p/p2p/host/basic"
	ma "gx/ipfs/QmUAQaWbKxGCUTuoQVvvicbQNZ9APF5pDGWyAZSe93AtKH/go-multiaddr"
	host "gx/ipfs/QmWf338UyG5DKyemvoFiomDPtkVNHLsw3GAt9XXHX5ZtsM/go-libp2p-host"
	pstore "gx/ipfs/QmXXCcQ7CLg5a81Ui9TTR35QcR4y7ZyihxwfjqaHfUVcVo/go-libp2p-peerstore"
	testutil "gx/ipfs/QmaEcA713Y54EtSsj7ZYfwXmsTfxrJ4oywr1iFt1d6LKY5/go-testutil"
	swarm "gx/ipfs/QmcjMKTqrWgMMCExEnwczefhno5fvx7FHDV63peZwDzHNF/go-libp2p-swarm"
	net "gx/ipfs/QmdysBu77i3YaagNtMAjiCJdeWWvds18ho5XEB784guQ41/go-libp2p-net"
	peer "gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"
	ci "gx/ipfs/QmfWDLQjGjVe4fr5CoztYW2DYYjRysMJrFe1RCsXLPTf46/go-libp2p-crypto"

	natinfo "github.com/whyrusleeping/natest/natinfo"
)

func getIdentity() (peer.ID, ci.PrivKey) {
	fi, err := os.Open("natest.key")
	if err != nil {
		ident, err := testutil.RandIdentity()
		if err != nil {
			panic(err)
		}

		data, err := ident.PrivateKey().Bytes()
		if err != nil {
			panic(err)
		}

		fi, err := os.Create("natest.key")
		if err != nil {
			panic(err)
		}

		_, err = fi.Write(data)
		if err != nil {
			panic(err)
		}

		fi.Close()

		return ident.ID(), ident.PrivateKey()
	}

	data, err := ioutil.ReadAll(fi)
	if err != nil {
		panic(err)
	}

	privk, err := ci.UnmarshalPrivateKey(data)
	if err != nil {
		panic(err)
	}

	pid, err := peer.IDFromPrivateKey(privk)
	if err != nil {
		panic(err)
	}

	return pid, privk
}

// create a 'Host' with a random peer to listen on the given address
func makebasicHost(listen string) (host.Host, error) {
	addr, err := ma.NewMultiaddr(listen)
	if err != nil {
		return nil, err
	}

	ps := pstore.NewPeerstore()
	var pid peer.ID

	pid, privk := getIdentity()
	ps.AddPrivKey(pid, privk)
	ps.AddPubKey(pid, privk.GetPublic())

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
	listenF := flag.Int("l", 7777, "serve http interface on given port")
	flag.Parse()

	var ha host.Host
	http.HandleFunc("/peerinfo", func(w http.ResponseWriter, r *http.Request) {
		var out []string
		for _, a := range ha.Addrs() {
			out = append(out, a.String())
		}
		pi := map[string]interface{}{
			"ID":    ha.ID().Pretty(),
			"Addrs": out,
		}

		json.NewEncoder(w).Encode(pi)
	})
	go func() {
		panic(http.ListenAndServe(fmt.Sprintf(":%d", *listenF), nil))
	}()

	var err error
	ha, err = makebasicHost("/ip4/0.0.0.0/tcp/0")
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

		resp, err := makeResp(ha, &req, s.Conn().RemoteMultiaddr())
		if err != nil {
			fmt.Println(err)
			return
		}

		resp.SeenAddr = s.Conn().RemoteMultiaddr().String()

		err = json.NewEncoder(s).Encode(&resp)
		if err != nil {
			fmt.Println(err)
			return
		}
	})

	select {}
}

func makeResp(h host.Host, req *natinfo.NATRequest, connaddr ma.Multiaddr) (*natinfo.NATResponse, error) {
	pid, err := peer.IDB58Decode(req.PeerID)
	if err != nil {
		return nil, err
	}

	var addrs []ma.Multiaddr
	laddr, err := ma.NewMultiaddr(req.ListenAddr)
	if err != nil {
		return nil, err
	}
	addrs = append(addrs, laddr)

	if req.PortMapped != "" {
		extaddr, err := ma.NewMultiaddr(req.PortMapped)
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, extaddr)
	}

	port, err := laddr.ValueForProtocol(ma.P_TCP)
	if err != nil {
		return nil, err
	}

	ipaddr, err := connaddr.ValueForProtocol(ma.P_IP4)
	if err != nil {
		return nil, err
	}

	hopeful, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%s", ipaddr, port))
	if err != nil {
		return nil, err
	}
	addrs = append(addrs, hopeful)

	pinfo := pstore.PeerInfo{
		ID:    pid,
		Addrs: addrs,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	err = h.Connect(ctx, pinfo)
	if err != nil {
		return &natinfo.NATResponse{
			ConnectBackSuccess: false,
			ConnectBackMsg:     err.Error(),
		}, nil
	}

	conns := h.Network().ConnsToPeer(pid)
	return &natinfo.NATResponse{
		ConnectBackSuccess: true,
		ConnectBackAddr:    conns[0].RemoteMultiaddr().String(),
	}, nil
}
