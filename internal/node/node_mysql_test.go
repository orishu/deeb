package node

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/orishu/deeb/internal/backend/mysql"
	"github.com/orishu/deeb/internal/lib"
	internaltesting "github.com/orishu/deeb/internal/lib/testing"
	"github.com/orishu/deeb/internal/transport"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func Test_mysql_cluster_with_in_process_transport(t *testing.T) {
	ctx := context.Background()

	logger := lib.NewDevelopmentLogger()
	kubeHelper, err := lib.NewKubeHelper("test", logger)
	require.NoError(t, err)

	err = kubeHelper.EnsureSecret(ctx, "my-ssh-key", internaltesting.SSHSecretSpec)
	require.NoError(t, err)
	err = kubeHelper.EnsureConfigMap(ctx, "t1-deeb-configuration", internaltesting.ConfigMapSpec)
	require.NoError(t, err)
	err = kubeHelper.EnsureStatefulSet(ctx, "t1-deeb", statefulSetSpec)
	require.NoError(t, err)
	defer func() {
		kubeHelper.DeleteStatefulSet(ctx, "t1-deeb")
		kubeHelper.DeletePeristentVolumeClaims(ctx, "t1", "deeb")
	}()

	privKey, err := lib.ExtractBytesFromSecretYAML(internaltesting.SSHSecretSpec, "id_rsa")
	require.NoError(t, err)

	inprocessReg := transport.NewInProcessRegistry()
	nodeInfos := []NodeInfo{
		NodeInfo{Addr: "localhost", Port: "10000"},
		NodeInfo{Addr: "localhost", Port: "10001"},
		NodeInfo{Addr: "localhost", Port: "10002"},
	}
	nodeParams := createNodeParams(nodeInfos)
	nodes, closer := createMySQLNodes(ctx, kubeHelper, t, privKey, nodeParams, inprocessReg, logger)
	defer closer()

	err = nodes[0].Start(ctx)
	require.NoError(t, err)

	time.Sleep(2 * time.Second)
	nodes[0].AddNode(ctx, 101, nodeInfos[1])

	err = nodes[1].Start(ctx)
	require.NoError(t, err)

	time.Sleep(2 * time.Second)
	nodes[1].AddNode(ctx, 102, nodeInfos[2])

	err = nodes[2].Start(ctx)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	err = nodes[1].WriteQuery(ctx, "CREATE TABLE testdb.table1 (f1 INT, f2 TEXT)")
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	err = nodes[1].WriteQuery(ctx, `INSERT INTO testdb.table1 (f1, f2) VALUES (10, "ten"), (20, "twenty")`)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Stop node0
	nodes[0].Stop()
	time.Sleep(2 * time.Second)

	// Propose data while node0 is down
	err = nodes[2].WriteQuery(ctx, `INSERT INTO testdb.table1 (f1, f2) VALUES (30, "thirty")`)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Restart node0
	newNode0, closer2 := createMySQLNode(ctx, t, kubeHelper, "t1-deeb-0", privKey, NodeParams{NodeID: 100, AddrPort: nodeInfos[0]}, inprocessReg, logger)
	nodes[0] = newNode0
	defer closer2()
	err = nodes[0].Restart(ctx)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Check that node0 got the new row that was added while it was down
	rows, err := nodes[0].ReadQuery(ctx, `SELECT f2 FROM testdb.table1 WHERE f1 = 30`)
	require.True(t, rows.Next())
	var value string
	err = rows.Scan(&value)
	require.NoError(t, err)
	require.Equal(t, "thirty", value)
	require.False(t, rows.Next())

	time.Sleep(1 * time.Second)

	// Propose some more data
	err = nodes[2].WriteQuery(ctx, `INSERT INTO testdb.table1 (f1, f2) VALUES (40, "forty")`)
	require.NoError(t, err)
	err = nodes[2].WriteQuery(ctx, `INSERT INTO testdb.table1 (f1, f2) VALUES (50, "fifty")`)
	require.NoError(t, err)
	err = nodes[2].WriteQuery(ctx, `INSERT INTO testdb.table1 (f1, f2) VALUES (60, "sixty")`)
	require.NoError(t, err)

	// Stop and delete the data from Node 2, have a new MySQL pod come up
	// and see a new node start with a snapshot.
	nodes[2].Stop()
	time.Sleep(2 * time.Second)
	node2SSHPort := nodes[2].backend.(*mysql.Backend).SSHPort()
	session, _, err := lib.MakeSSHSession("localhost", node2SSHPort, "mysql", privKey)
	require.NoError(t, err)
	err = session.Run("rm -rf /var/lib/mysql/*")
	require.NoError(t, err)

	err = kubeHelper.DeletePod(ctx, "t1-deeb-2")
	require.NoError(t, err)
	time.Sleep(2 * time.Second)
	// Wait for pod to be gone
	err = kubeHelper.WaitForPodToBeGone(ctx, "t1-deeb-2", 120)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Remove the old node from the topology
	err = nodes[0].RemoveNode(ctx, 102)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Register the new Node 2 (ID=103) with the transport manager
	inprocessReg.Register(nodes[2].transportMgr, nodeInfos[2].Addr, nodeInfos[2].Port, 103)

	// Add node to topology
	nodes[0].AddNode(ctx, 103, nodeInfos[2])
	time.Sleep(2 * time.Second)

	// Wait for pod to be running+ready
	err = kubeHelper.WaitForPodToBeReady(ctx, "t1-deeb-2", 120)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Start the new Node 2 with ID 103
	node2Params := NodeParams{
		NodeID:         103,
		AddrPort:       nodeInfos[2],
		PotentialPeers: nodeInfos[:2],
	}
	newNode2, closer3 := createMySQLNode(ctx, t, kubeHelper, "t1-deeb-2", privKey, node2Params, inprocessReg, logger)
	nodes[2] = newNode2
	defer closer3()
	err = nodes[2].Start(ctx)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Wait for the new Node 2 to have the new rows that were part of the snapshot
	attempts := 100
	for ; attempts > 0; attempts-- {
		rows, err := nodes[2].ReadQuery(ctx, `SELECT f2 FROM testdb.table1 WHERE f1 = 60`)
		if err != nil {
			logger.Warnf("new node2 not yet ready, err=%+v", err)
			time.Sleep(10 * time.Second)
			continue
		}
		if !rows.Next() {
			logger.Warnf("new node2 not yet ready, data not there yet")
			time.Sleep(10 * time.Second)
			continue
		}
		err = rows.Scan(&value)
		require.NoError(t, err)
		require.Equal(t, "sixty", value)
		require.False(t, rows.Next())
		break
	}
	require.NotEqual(t, attempts, 0)

	nodes[0].Stop()
	nodes[1].Stop()
	nodes[2].Stop()

	time.Sleep(2 * time.Second)
}

func createMySQLNodes(
	ctx context.Context,
	kubeHelper *lib.KubeHelper,
	t *testing.T,
	privKey []byte,
	nodeParams []NodeParams,
	inprocessReg *transport.InProcessRegistry,
	logger *zap.SugaredLogger,
) ([]*Node, func()) {
	nodes := make([]*Node, 0, len(nodeParams))
	closers := make([]func(), 0, len(nodeParams))
	for i, np := range nodeParams {
		podName := fmt.Sprintf("t1-deeb-%d", i)
		err := kubeHelper.WaitForPodToBeReady(ctx, podName, 60)
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		n, closer := createMySQLNode(ctx, t, kubeHelper, podName, privKey, np, inprocessReg, logger)
		nodes = append(nodes, n)
		closers = append(closers, closer)
	}
	closer := func() {
		for _, c := range closers {
			c()
		}
	}
	return nodes, closer
}

func createMySQLNode(
	ctx context.Context,
	t *testing.T,
	kubeHelper *lib.KubeHelper,
	podName string,
	privKey []byte,
	np NodeParams,
	inprocessReg *transport.InProcessRegistry,
	logger *zap.SugaredLogger,
) (*Node, func()) {
	transportMgr := transport.NewTransportManager(transport.NewInProcessClientFactory(inprocessReg))
	inprocessReg.Register(transportMgr, np.AddrPort.Addr, np.AddrPort.Port, np.NodeID)

	ports, err := freeport.GetFreePorts(2)
	require.NoError(t, err)
	mysqlPort := ports[0]
	sshPort := ports[1]

	portForwardCloser1, err := kubeHelper.PortForward(podName, mysqlPort, 3306)
	require.NoError(t, err)
	portForwardCloser2, err := kubeHelper.PortForward(podName, sshPort, 22)
	require.NoError(t, err)

	time.Sleep(3 * time.Second)

	be, st := mysql.New(mysql.Params{
		EntriesToRetain: 5,
		Addr:            "localhost",
		MysqlPort:       mysqlPort,
		SSHPort:         sshPort,
		PrivateKey:      privKey,
	}, logger)

	peerMgr := transport.NewPeerManager(transportMgr)
	attempts := 80
	for ; attempts > 0; attempts-- {
		err = be.Start(ctx)
		if err == nil {
			break
		}
		logger.Warnf("backend not up yet, error: %+v", err)
		time.Sleep(5 * time.Second)
	}
	require.NoError(t, err)
	closer := func() {
		be.Stop(ctx)
		portForwardCloser1()
		portForwardCloser2()
	}
	return New(np, peerMgr, transportMgr, st, be, logger), closer
}
