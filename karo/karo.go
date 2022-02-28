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
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"log"
	"net"
	"sync"
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

func (r *Reactor) Initialize() {
	config := r.SetOptions()

	r.Host, err = libp2p.New(config)

	r.Blacklist = new(BlackList)
	r.Filters = new(KaroBloom)
	ctx, cancel := context.WithCancel(nil)
	r.context = ctx
	defer cancel()

}

func (r Reactor) SetOptions() libp2p.Option {

	cmgr, _ := connmgr.NewConnManager(
		100, // Lowwater
		400, // HighWater,
	)

	return libp2p.ChainOptions(libp2p.EnableRelayService(), libp2p.EnableNATService(), //nolint:typecheck
		libp2p.EnableRelay(), libp2p.DefaultMuxers, libp2p.DefaultTransports, libp2p.DefaultSecurity,
		libp2p.ConnectionManager(cmgr)) //nolint:typecheck

}
