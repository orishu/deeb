package node

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/orishu/deeb/internal/backend/sqlite"
	"github.com/orishu/deeb/internal/lib"
	"github.com/orishu/deeb/internal/transport"
	"github.com/stretchr/testify/require"
)

func Test_cluster_operation_with_in_process_transport(t *testing.T) {
	logger := lib.NewDevelopmentLogger()
	inprocessReg := transport.NewInProcessRegistry()
	dir, err := ioutil.TempDir(".", fmt.Sprintf("%s-*", t.Name()))
	require.NoError(t, err)
	defer os.RemoveAll(dir)

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
	for i, np := range nodeParams {
		transportMgr := transport.NewTransportManager(transport.NewInProcessClientFactory(inprocessReg))
		inprocessReg.Register(transportMgr, np.AddrPort.Addr, np.AddrPort.Port, np.NodeID)
		nodeDir := fmt.Sprintf("%s/db%d", dir, i)
		os.Mkdir(nodeDir, 0755)
		be, st := sqlite.New(nodeDir, logger)
		n := New(
			np,
			transport.NewPeerManager(transportMgr),
			transportMgr,
			st,
			be,
			logger,
		)
		nodes = append(nodes, n)
	}

	ctx := context.Background()

	go func() {
		err := nodes[0].Start(ctx)
		require.NoError(t, err)
	}()

	time.Sleep(1 * time.Second)
	nodes[0].AddNode(ctx, 101, nodeInfos[1])

	go func() {
		err := nodes[1].Start(ctx)
		require.NoError(t, err)
	}()

	time.Sleep(1 * time.Second)
	nodes[1].AddNode(ctx, 102, nodeInfos[2])

	go func() {
		err := nodes[2].Start(ctx)
		require.NoError(t, err)
	}()

	time.Sleep(1 * time.Second)

	nodes[1].Propose(ctx, []byte("some data proposed by node1"))
	time.Sleep(1 * time.Second)
	nodes[2].Propose(ctx, []byte("some data proposed by node2"))
	time.Sleep(1 * time.Second)

	require.Equal(t, uint64(100), nodes[0].GetID())
	nodes[2].Stop(ctx)
	nodes[1].Stop(ctx)
	nodes[0].Stop(ctx)
}
