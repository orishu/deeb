package lib

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ChannelPool_basic(t *testing.T) {
	cp := NewChannelPool()
	done := make(chan bool)

	id1, ch1 := cp.GetNewChannel()
	go func() {
		value := <-ch1
		require.Equal(t, "my error", value.Error())
		done <- true
	}()

	ch := cp.Remove(id1)
	ch <- errors.New("my error")

	_ = <-done
}
