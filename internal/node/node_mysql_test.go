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
	defer kubeHelper.DeleteStatefulSet(ctx, "t1-deeb")

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
		err := kubeHelper.WaitForPodToBeReady(ctx, podName, 30)
		require.NoError(t, err)
		time.Sleep(time.Second)

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

	be, st := mysql.New(mysql.Params{
		EntriesToRetain: 5,
		Addr:            "localhost",
		MysqlPort:       mysqlPort,
		SSHPort:         sshPort,
		PrivateKey:      privKey,
	}, logger)

	peerMgr := transport.NewPeerManager(transportMgr)
	err = be.Start(ctx)
	require.NoError(t, err)
	closer := func() {
		be.Stop(ctx)
		portForwardCloser1()
		portForwardCloser2()
	}
	return New(np, peerMgr, transportMgr, st, be, logger), closer
}
