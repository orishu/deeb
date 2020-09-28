package lib

import (
	"sync"
	"sync/atomic"
)

// ChannelPool tracks active channels by ID, so channels can be associated with
// an integer and found by their ID.
type ChannelPool struct {
	pending map[uint64](chan int)
	mutex   sync.Mutex
	nextID  uint64
}

// NewChannelPool returns a new ChannelPool
func NewChannelPool() *ChannelPool {
	return &ChannelPool{pending: make(map[uint64](chan int))}
}

// GetNewChannel allocates a new channel associated with an ID
func (cp *ChannelPool) GetNewChannel() (uint64, chan int) {
	ch := make(chan int)
	id := atomic.AddUint64(&cp.nextID, 1)
	cp.mutex.Lock()
	defer cp.mutex.Unlock()
	cp.pending[id] = ch
	return id, ch
}

// Remove removes a channel by its ID, returning the channel.
func (cp *ChannelPool) Remove(channelID uint64) chan int {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()
	ch, ok := cp.pending[channelID]
	if ok {
		delete(cp.pending, channelID)
	}
	return ch
}
