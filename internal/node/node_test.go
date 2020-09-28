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
	"go.uber.org/zap"
)

func Test_cluster_operation_with_in_process_transport(t *testing.T) {
	logger := lib.NewDevelopmentLogger()
	dir, err := ioutil.TempDir(".", fmt.Sprintf("%s-*", t.Name()))
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	inprocessReg := transport.NewInProcessRegistry()
	nodeInfos := []NodeInfo{
		NodeInfo{Addr: "localhost", Port: "10000"},
		NodeInfo{Addr: "localhost", Port: "10001"},
		NodeInfo{Addr: "localhost", Port: "10002"},
	}
	nodeParams := createNodeParams(nodeInfos)
	nodes := createNodes(t, dir, nodeParams, inprocessReg, logger)

	ctx := context.Background()

	err = nodes[0].Start(ctx)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)
	nodes[0].AddNode(ctx, 101, nodeInfos[1])

	err = nodes[1].Start(ctx)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)
	nodes[1].AddNode(ctx, 102, nodeInfos[2])

	err = nodes[2].Start(ctx)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	err = nodes[1].WriteQuery(ctx, "some data1 proposed by node1")
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	err = nodes[1].WriteQuery(ctx, "some data2 proposed by node1")
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Stop node0
	nodes[0].Stop(ctx)
	time.Sleep(2 * time.Second)

	// Propose data while node0 is down
	err = nodes[2].WriteQuery(ctx, "some data3 proposed by node2")
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Restart node0 with the existing directory db0
	node0Dir := fmt.Sprintf("%s/db0", dir)
	nodes[0] = createNode(node0Dir, NodeParams{NodeID: 100, AddrPort: nodeInfos[0]}, inprocessReg, logger)
	err = nodes[0].Restart(ctx)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	require.Equal(t, uint64(100), nodes[0].GetID())
	nodes[2].Stop(ctx)
	nodes[1].Stop(ctx)
	nodes[0].Stop(ctx)
}

func createNodeParams(nodeInfos []NodeInfo) []NodeParams {
	return []NodeParams{
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
}

func createNodes(
	t *testing.T,
	dir string,
	nodeParams []NodeParams,
	inprocessReg *transport.InProcessRegistry,
	logger *zap.SugaredLogger,
) []*Node {
	nodes := make([]*Node, 0, len(nodeParams))
	for i, np := range nodeParams {
		nodeDir := fmt.Sprintf("%s/db%d", dir, i)
		n := createNode(nodeDir, np, inprocessReg, logger)
		nodes = append(nodes, n)
	}
	return nodes
}

func createNode(
	nodeDir string,
	np NodeParams,
	inprocessReg *transport.InProcessRegistry,
	logger *zap.SugaredLogger,
) *Node {
	transportMgr := transport.NewTransportManager(transport.NewInProcessClientFactory(inprocessReg))
	inprocessReg.Register(transportMgr, np.AddrPort.Addr, np.AddrPort.Port, np.NodeID)
	os.Mkdir(nodeDir, 0755)
	be, st := sqlite.New(nodeDir, logger)
	peerMgr := transport.NewPeerManager(transportMgr)
	return New(np, peerMgr, transportMgr, st, be, logger)
}
