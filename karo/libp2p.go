package karo

import (
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
)

/*
	KARO is the name of the quantos library for the peer-to-peer network
*/

type Node struct {
	Host      host.Host
	PeerStore peerstore.Peerstore
	identity  peer.ID
}
