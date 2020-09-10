package transport

import (
	"context"
	"sync"
)

// PeerParams is the collection of parameters needed for setting up a peer
// client.
type PeerParams struct {
	NodeID uint64
	Addr   string
	Port   string
}

// Peer maintains the peer metadata and connection.
type Peer struct {
	client Client
}

// PeerManager maintains the set of peers.
type PeerManager struct {
	mutex        sync.RWMutex
	peerMap      map[uint64]Peer
	transportMgr *TransportManager
}

// NewPeerManager returns a new PeerManager.
func NewPeerManager(transportMgr *TransportManager) *PeerManager {
	return &PeerManager{
		peerMap:      map[uint64]Peer{},
		transportMgr: transportMgr,
	}
}

// UpsertPeer upserts a new peer to the set of manager peers.
func (pm PeerManager) UpsertPeer(ctx context.Context, params PeerParams) error {
	c, err := pm.transportMgr.CreateClient(ctx, params.Addr, params.Port)
	if err != nil {
		return err
	}
	p := Peer{client: c}
	pm.mutex.Lock()
	if _, ok := pm.peerMap[params.NodeID]; ok {
		// Close an old client in case it's an upsert.
		defer p.close()
	}
	pm.peerMap[params.NodeID] = p
	pm.mutex.Unlock()
	return nil
}

// RemovePeer removes a peer from the tracked set.
func (pm PeerManager) RemovePeer(ctx context.Context, nodeID uint64) {
	pm.mutex.Lock()
	if p, ok := pm.peerMap[nodeID]; ok {
		defer p.close()
		delete(pm.peerMap, nodeID)
	}
	pm.mutex.Unlock()
}

// ClientForPeer returns the registered gRPC client for the node ID, or nil if
// none.
func (pm PeerManager) ClientForPeer(nodeID uint64) Client {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	if p, ok := pm.peerMap[nodeID]; ok {
		return p.client
	}
	return nil
}

// Close returns the first error received when closing clients.
func (pm PeerManager) Close() error {
	var err error
	pm.mutex.Lock()
	for _, p := range pm.peerMap {
		e := p.close()
		if e != nil && err == nil {
			err = e
		}
	}
	pm.peerMap = make(map[uint64]Peer)
	pm.mutex.Unlock()
	return err
}

func (p Peer) close() error {
	if p.client == nil {
		return nil
	}
	return p.client.Close()
}
