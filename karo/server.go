package karo

import (
	"encoding/hex"
	"fmt"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	libp2ptls "github.com/libp2p/go-libp2p-tls"
	ma "github.com/multiformats/go-multiaddr"
	"quantos_reactor/karo/vault"
	"sync"
)

type ServerConfig struct {
}

// KaroServer is what could be commonly refered as a master node
type KaroServer struct {
	logger           hclog.Logger
	config           *ServerConfig
	quitCh           chan struct{}
	host             host.Host
	addrs            []ma.Multiaddr
	peers            map[peer.ID]*Peer
	peersLock        sync.Mutex
	metrics          *Metrics
	identity         *libp2ptls.Identity
	pubsub           pubsub.PubSub
	discovery        *discovery.Discovery
	protocols        map[string]Protocol
	protocolsLock    sync.Mutex
	secretsManager   vault.Manager
	joinWatchers     map[peer.ID]chan error
	joinWatchersLock sync.Mutex

	emitterPeerEvent event.Emitter

	inboundConnCount int64
}

type Peer struct {
	isMaster      bool
	masterSrv     *KaroServer
	Info          peer.AddrInfo
	ConnDirection network.Direction
}

func getLibP2PKey(secretsManager *vault.Manager) (crypto.PrivKey, error) {
	var key crypto.PrivKey
	libp2pKey, libp2pKeyEncoded, keyErr := generateAndEncodeLibP2PKey()
	if keyErr != nil {
		return nil, fmt.Errorf("unable to generate networking private key for Secrets Manager, %w", keyErr)
	}

	// Write the networking private key to disk
	if setErr := secretsManager.SetSecret(NetworkKey, libp2pKeyEncoded); setErr != nil {
		return nil, fmt.Errorf("unable to store networking private key to Secrets Manager, %w", setErr)
	}

	key = libp2pKey
	return key, nil
}

func generateAndEncodeLibP2PKey() (crypto.PrivKey, []byte, error) {
	priv, _, err := crypto.GenerateKeyPair(crypto.Secp256k1, 256)
	if err != nil {
		return nil, nil, err
	}
	buf, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	return priv, []byte(hex.EncodeToString(buf)), nil
}

func parseLibP2PKey(key []byte) (crypto.PrivKey, error) {
	buf, err := hex.DecodeString(string(key))
	if err != nil {
		return nil, err
	}

	libp2pKey, err := crypto.UnmarshalPrivateKey(buf)
	if err != nil {
		return nil, err
	}

	return libp2pKey, nil
}
