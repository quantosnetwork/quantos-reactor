package karo

import (
	"context"
	"fmt"

	"github.com/bits-and-blooms/bloom/v3"
	ds "github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/protocol"
	router "github.com/libp2p/go-libp2p-core/routing"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	noise "github.com/libp2p/go-libp2p-noise"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"

	"log"
	"net"
	"sync"
	"time"

	mplex "github.com/libp2p/go-libp2p-mplex"
	libp2ptls "github.com/libp2p/go-libp2p-tls"
	yamux "github.com/libp2p/go-libp2p-yamux"
)

const ns = "/karo/net"
const keyPeer = "/peer"
const keyAddr = "/addr"
const keySubnet = "/subnet"
const protocolID = protocol.ID("/karo/net/v1/")

type KaroInterface interface {
	Initialize()
	GetReactor() *Reactor
	AddPeer(peer.ID)
	RemovePeer(peer.ID)
	BanPeer(peer.ID)
	Bootstrap(peers []string)
	Connect(h host.Host)
}

type Reactor struct {
	ID        peer.ID
	Host      host.Host
	Bus       event.Bus
	PeerStore peerstore.Peerstore
	Incoming  []peer.ID
	Outgoing  []peer.ID
	Blacklist *BlackList
	Datastore ds.Datastore
	context   context.Context
	Filters   *KaroBloom
	DHT       *dht.IpfsDHT
	GossipSub *pubsub.PubSub
	KaroInterface
}

type KaroBloom struct {
	mu        sync.Mutex
	recent    bloom.BloomFilter
	itemCount int
}

type BlackList struct {
	mu sync.RWMutex

	blockedPeers   map[peer.ID]struct{}
	blockedAddrs   map[string]struct{}
	blockedSubnets map[string]*net.IPNet
}

func (r *Reactor) Connect(h host.Host) {
	pi := r.PeerStore.PeerInfo(h.ID())
	err = h.Connect(r.context, pi)
	if err != nil {
		log.Fatal(err)
	}
}

var err error

var reactor Reactor

func (r *Reactor) Initialize(n *Node, ctx context.Context, cancel func()) {
	defer cancel()
	newDHT := func(h host.Host) (router.PeerRouting, error) {
		var err error
		dht, err := dht.New(ctx, h)
		return dht, err
	}
	routing := libp2p.Routing(newDHT)

	priv, _, err := crypto.GenerateKeyPair(
		crypto.Ed25519, // Select your key type. Ed25519 are nice short
		-1,             // Select key length when possible (i.e. RSA).
	)
	if err != nil {
		panic(err)
	}

	security := libp2p.ChainOptions(
		libp2p.Security(libp2ptls.ID, libp2ptls.New),
		libp2p.Security(noise.ID, noise.New),
	)

	listenAddrs := libp2p.ListenAddrStrings(
		"/ip4/0.0.0.0/tcp/0",
		"/ip4/0.0.0.0/tcp/0/ws",
	)

	rhost, err := libp2p.New(libp2p.Identity(priv), listenAddrs, security, libp2p.DefaultTransports,
		libp2p.NATPortMap(), routing, libp2p.EnableAutoRelay(), libp2p.EnableNATService())
	if err != nil {
		panic(err)
	}

	id := rhost.ID().Pretty()
	fmt.Println(id)

	//r.Blacklist = new(BlackList)

	/*_, err = r.InitDHT(ctx, rhost, r.DHT.Bootstrap())
	if err != nil {
		log.Fatal(err)
		return
	}*/
	ps, err := pubsub.NewGossipSub(ctx, rhost)
	if err != nil {
		panic(err)
	}
	r.GossipSub = ps
	var wg sync.WaitGroup
	defer wg.Done()
	wg.Add(1)
	go func() {
		Discover(ctx, rhost, "karov1-blockchain")
	}()
	wg.Wait()

}

func (r *Reactor) InitDHT(ctx context.Context, host host.Host, bootstrapPeers []ma.Multiaddr) (*discovery.
	RoutingDiscovery, error) {
	var options []dht.Option
	var wg sync.WaitGroup

	if len(bootstrapPeers) == 0 {
		options = append(options, dht.Mode(dht.ModeServer))
	}
	r.DHT, err = dht.New(ctx, r.Host, options...)
	if err != nil {
		panic(err)
	}
	if err = r.DHT.Bootstrap(ctx); err != nil {
		return nil, err
	}

	for _, peerAddr := range bootstrapPeers {
		pi, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := host.Connect(ctx, *pi); err != nil {
				log.Printf("Error while connecting to node %q: %-v", pi, err)
			}
		}()
		wg.Wait()
		return discovery.NewRoutingDiscovery(r.DHT), nil
	}
	return nil, nil
}

func Discover(ctx context.Context, h host.Host, rendezvous string) {

	bootstrapPeers := dht.DefaultBootstrapPeers
	var options []dht.Option
	if len(bootstrapPeers) == 0 {
		log.Println("starting server only mode...")
		options = append(options, dht.Mode(dht.ModeServer))
	}
	DHT, _ := dht.New(ctx, h, options...)
	routingDiscovery := discovery.NewRoutingDiscovery(DHT)
	discovery.Advertise(ctx, routingDiscovery, rendezvous)
	ticker := time.NewTicker(time.Second * 1)
	defer ticker.Stop()
	log.Println(ticker.C)
	if err = DHT.Bootstrap(ctx); err != nil {
		panic(err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			peers, err := discovery.FindPeers(ctx, routingDiscovery, rendezvous)
			//log.Println(peers[0].ID.String())

			if err != nil {
				log.Fatal(err)
			}
			for _, p := range peers {

				if p.ID == h.ID() {
					continue
				}
				log.Println(h.Network().Connectedness(p.ID))
				if h.Network().Connectedness(p.ID) != network.Connected {
					_, err = h.Network().DialPeer(ctx, p.ID)
					if err != nil {
						continue
					}
				}
			}
		}
	}
}

func (r Reactor) SetOptions() []libp2p.Option {

	mgr, _ := connmgr.NewConnManager(
		100, // Lowwater
		400, // HighWater,
	)
	cmgr := libp2p.ConnectionManager(mgr)
	newDHT := func(h host.Host) (router.PeerRouting, error) {
		var err error
		d := r.DHT
		return d, err
	}
	routing := libp2p.Routing(newDHT)

	transports := libp2p.DefaultTransports

	muxers := libp2p.ChainOptions(
		libp2p.Muxer("/yamux/1.0.0", yamux.DefaultTransport),
		libp2p.Muxer("/mplex/6.7.0", mplex.DefaultTransport),
	)

	security := libp2p.Security(libp2ptls.ID, libp2ptls.New)

	listenAddrs := libp2p.ListenAddrStrings(
		"/ip4/0.0.0.0/tcp/0",
		"/ip4/0.0.0.0/tcp/0/ws",
	)
	var opt []libp2p.Option
	opt = append(opt, cmgr, routing, transports, muxers, security, listenAddrs)

	return opt

}

func (r *Reactor) SetPubSub(h host.Host) {

}
