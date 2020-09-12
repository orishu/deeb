package node

import (
	"context"
	"testing"
	"time"

	"github.com/orishu/deeb/internal/lib"
	"github.com/orishu/deeb/internal/transport"
	"github.com/stretchr/testify/require"
)

func Test_cluster_operation_with_in_process_transport(t *testing.T) {
	logger := lib.NewDevelopmentLogger()
	inprocessReg := transport.NewInProcessRegistry()

	nodeInfos := []NodeInfo{
		NodeInfo{Addr: "localhost", Port: "10000"},
		NodeInfo{Addr: "localhost", Port: "10001"},
		NodeInfo{Addr: "localhost", Port: "10002"},
	}
	nodeParams := []NodeParams{
		{
			NodeID:         100,
			AddrPort:       nodeInfos[0],
			IsNewCluster:   true,
			PotentialPeers: []NodeInfo{nodeInfos[1], nodeInfos[2]},
		},
		{
			NodeID:         101,
			AddrPort:       nodeInfos[1],
			PotentialPeers: []NodeInfo{nodeInfos[0], nodeInfos[2]},
		},
		{
			NodeID:         102,
			AddrPort:       nodeInfos[2],
			PotentialPeers: []NodeInfo{nodeInfos[0], nodeInfos[1]},
		},
	}
	nodes := make([]*Node, 0, len(nodeParams))
	for _, np := range nodeParams {
		transportMgr := transport.NewTransportManager(transport.NewInProcessClientFactory(inprocessReg))
		inprocessReg.Register(transportMgr, np.AddrPort.Addr, np.AddrPort.Port, np.NodeID)
		n := New(np, transport.NewPeerManager(transportMgr), transportMgr, logger)
		nodes = append(nodes, n)
	}

	ctx := context.Background()

	go func() {
		nodes[0].Start(ctx)
	}()

	time.Sleep(1 * time.Second)
	nodes[0].AddNode(ctx, 101, nodeInfos[1])

	go func() {
		nodes[1].Start(ctx)
	}()

	time.Sleep(1 * time.Second)
	nodes[1].AddNode(ctx, 102, nodeInfos[2])

	go func() {
		nodes[2].Start(ctx)
	}()

	time.Sleep(1 * time.Second)

	nodes[1].Propose(ctx, []byte("some data proposed by node1"))
	time.Sleep(1 * time.Second)
	nodes[2].Propose(ctx, []byte("some data proposed by node2"))
	time.Sleep(1 * time.Second)

	require.Equal(t, uint64(100), nodes[0].GetID())
	nodes[2].Stop()
	nodes[1].Stop()
	nodes[0].Stop()
}
