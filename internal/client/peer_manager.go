package client

import (
	"context"
	"fmt"
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
	client *Client
}

// PeerManager maintains the set of peers.
type PeerManager struct {
	mutex   sync.RWMutex
	peerMap map[uint64]Peer
}

// NewPeerManager returns a new PeerManager.
func NewPeerManager() *PeerManager {
	return &PeerManager{}
}

// AddPeer adds a new peer to the set of manager peers.
func (pm PeerManager) AddPeer(ctx context.Context, params PeerParams) error {
	pm.mutex.RLock()
	if _, ok := pm.peerMap[params.NodeID]; ok {
		pm.mutex.Unlock()
		return fmt.Errorf("peer ID already exists: %d", params.NodeID)
	}
	pm.mutex.RUnlock()
	c, err := newClient(ctx, params.Addr, params.Port)
	if err != nil {
		return err
	}
	p := Peer{client: c}
	pm.mutex.Lock()
	if _, ok := pm.peerMap[params.NodeID]; ok {
		// Handle an unlikely race that a peer with the same ID was
		// added in between the two lock sessions.
		pm.mutex.Unlock()
		_ = p.close()
		return fmt.Errorf("peer ID already exists (race): %d", params.NodeID)
	}
	pm.peerMap[params.NodeID] = p
	pm.mutex.Unlock()
	return nil
}

// ClientForPeer returns the registered gRPC client for the node ID, or nil if
// none.
func (pm PeerManager) ClientForPeer(nodeID uint64) *Client {
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
	pm.peerMap = nil
	pm.mutex.Unlock()
	return err
}

func (p Peer) close() error {
	if p.client == nil {
		return nil
	}
	return p.client.Close()
}
