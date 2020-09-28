package lib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ChannelPool_basic(t *testing.T) {
	cp := NewChannelPool()
	done := make(chan bool)

	id1, ch1 := cp.GetNewChannel()
	go func() {
		value := <-ch1
		require.Equal(t, 100, value)
		done <- true
	}()

	ch := cp.Remove(id1)
	ch <- 100

	_ = <-done
}
