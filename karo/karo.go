package karo

import (
	"context"
	bloom "github.com/bits-and-blooms/bloom/v3"
	ds "github.com/ipfs/go-datastore"
	libp2p "github.com/libp2p/go-libp2p" //nolint:typecheck
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/protocol"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"
	router "github.com/libp2p/go-libp2p-core/routing"
	"log"
	"net"
	"sync"
	"time"
)

const ns = "/karo/net"
const keyPeer = "/peer"
const keyAddr = "/addr"
const keySubnet = "/subnet"
const protocolID = protocol.ID("/karo/net/v1/")

type Karo interface {
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
	Stream    network.Stream
	Conns     []network.Conn
	PeerStore peerstore.Peerstore
	Incoming  []peer.ID
	Outgoing  []peer.ID
	Blacklist *BlackList
	Datastore ds.Datastore
	context   context.Context
	Filters   *KaroBloom
	DHT       *dht.IpfsDHT
	GossipSub *pubsub.PubSub
	Karo
}

type KaroBloom struct {
	mu        sync.Mutex
	recent    bloom.BloomFilter
	itemCount int
}

type BlackList struct {
	sync.RWMutex

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

func (r *Reactor) Initialize(ctx context.Context, cancel func())  {
	config := r.SetOptions()

	r.Host, err = libp2p.New(config)

	r.Blacklist = new(BlackList)
	r.Filters = new(KaroBloom)

	r.context = ctx
	defer cancel()
	_, err := r.InitDHT(r.context, r.Host, nil)
	if err != nil {
		log.Fatal(err)
		return
	}
	r.SetPubSub(r.Host)
	go r.Discover(r.context, r.Host, "karov1-blockchain")


}

func (r *Reactor) InitDHT(ctx context.Context, host host.Host, bootstrapPeers []ma.Multiaddr) (*discovery.
	RoutingDiscovery, error) {
	var options []dht.Option
	var wg sync.WaitGroup

	if len(bootstrapPeers) == 0 {
		options = append(options, dht.Mode(dht.ModeServer))
	}
	r.DHT, _ = dht.New(ctx, r.Host, options...)
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

func (r *Reactor) Discover(ctx context.Context, h host.Host, rendezvous string) {
	routingDiscovery := discovery.NewRoutingDiscovery(r.DHT)
	discovery.Advertise(ctx, routingDiscovery, rendezvous)
	ticker := time.NewTicker(time.Second * 1)
	defer ticker.Stop()

	for {
		select {
			case <-ctx.Done():
				return
				case <-ticker.C:
					peers, err := discovery.FindPeers(ctx, routingDiscovery, rendezvous)
					if err != nil {
						log.Fatal(err)
					}
					for _, p := range peers {
						if p.ID == h.ID() || p.ID == r.ID {
							continue
						}
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

func (r Reactor) SetOptions() libp2p.Option {

	cmgr, _ := connmgr.NewConnManager(
		100, // Lowwater
		400, // HighWater,
	)
	newDHT := func(h host.Host) (router.PeerRouting, error) {
		var err error
		d := r.DHT
		return d, err
	}
	routing := libp2p.Routing(newDHT)

	return libp2p.ChainOptions(libp2p.EnableRelayService(), libp2p.EnableNATService(), //nolint:typecheck
		libp2p.EnableRelay(), routing, libp2p.DefaultMuxers, libp2p.DefaultTransports, libp2p.DefaultSecurity,
		libp2p.ConnectionManager(cmgr)) //nolint:typecheck

}

func (r *Reactor) SetPubSub(h host.Host) {
		ps, err := pubsub.NewGossipSub(r.context, h)
		if err != nil {
			panic(err)
		}
		r.GossipSub = ps
}