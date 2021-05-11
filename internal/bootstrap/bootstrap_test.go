package bootstrap

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/orishu/deeb/internal/lib"
	nd "github.com/orishu/deeb/internal/node"
	"github.com/orishu/deeb/internal/transport"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
)

func Test_bootstrap_new_node_in_existing_cluster(t *testing.T) {
	nodeIDDir, err := ioutil.TempDir(".", fmt.Sprintf("%s-*", t.Name()))
	require.NoError(t, err)
	defer os.RemoveAll(nodeIDDir)
	ctrl := gomock.NewController(t)
	mockClient := transport.NewMockClient(ctrl)
	mockClient.EXPECT().
		GetHighestID(gomock.Any()).Return(uint64(123), nil).AnyTimes()
	mockClient.EXPECT().
		AddNewPeer(gomock.Any(), gomock.Eq(uint64(124)), gomock.Eq("t1-deeb-1"), gomock.Eq("10000")).
		Return(nil)

	expectedBSI := BootstrapInfo{
		NodeID:       124,
		IsNewCluster: false,
		NodeName:     "t1-deeb-1",
		Peers: []nd.NodeInfo{
			nd.NodeInfo{Addr: "t1-deeb-0", Port: "10000"},
			nd.NodeInfo{Addr: "t1-deeb-2", Port: "10000"},
		},
	}

	options := makeTestOptions(t, expectedBSI, nodeIDDir, "t1-deeb-1", mockClient)

	app := fxtest.New(t, options)
	app.RequireStart()
	app.RequireStop()
}

func Test_bootstrap_node_id_file_exists(t *testing.T) {
	nodeIDDir, err := ioutil.TempDir(".", fmt.Sprintf("%s-*", t.Name()))
	defer os.RemoveAll(nodeIDDir)
	err = ioutil.WriteFile(nodeIDDir+"/node_id", []byte("201"), 0644)
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	mockClient := transport.NewMockClient(ctrl)

	expectedBSI := BootstrapInfo{
		NodeID:       201,
		IsNewCluster: false,
		NodeName:     "t1-deeb-1",
		Peers: []nd.NodeInfo{
			nd.NodeInfo{Addr: "t1-deeb-0", Port: "10000"},
			nd.NodeInfo{Addr: "t1-deeb-2", Port: "10000"},
		},
	}

	options := makeTestOptions(t, expectedBSI, nodeIDDir, "t1-deeb-1", mockClient)

	app := fxtest.New(t, options)
	app.RequireStart()
	app.RequireStop()
}

func Test_bootstrap_new_node_in_new_cluster(t *testing.T) {
	nodeIDDir, err := ioutil.TempDir(".", fmt.Sprintf("%s-*", t.Name()))
	require.NoError(t, err)
	defer os.RemoveAll(nodeIDDir)
	ctrl := gomock.NewController(t)
	mockClient := transport.NewMockClient(ctrl)
	mockClient.EXPECT().
		GetHighestID(gomock.Any()).Return(uint64(0), errors.New("some error")).AnyTimes()

	expectedBSI := BootstrapInfo{
		NodeID:       1,
		IsNewCluster: true,
		NodeName:     "t1-deeb-0",
		Peers: []nd.NodeInfo{
			nd.NodeInfo{Addr: "t1-deeb-1", Port: "10000"},
			nd.NodeInfo{Addr: "t1-deeb-2", Port: "10000"},
		},
	}

	options := makeTestOptions(t, expectedBSI, nodeIDDir, "t1-deeb-0", mockClient)

	app := fxtest.New(t, options)
	app.RequireStart()
	app.RequireStop()
}

func makeTestOptions(t *testing.T, expectedBSI BootstrapInfo, nodeIDDir string, nodeName string, mockClient transport.Client) fx.Option {
	invokeFunc := func(lc fx.Lifecycle, params Params) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				bsi, err := New(ctx, params)
				require.NoError(t, err)
				require.Equal(t, expectedBSI, bsi)
				return nil
			},
		})
	}

	provides := fx.Provide(
		lib.NewDevelopmentLogger,
		func() transport.ClientFactory {
			return func(context.Context, string, string) (transport.Client, error) {
				return mockClient, nil
			}
		},
		func(tcf transport.ClientFactory, logger *zap.SugaredLogger) Params {
			return Params{
				ClusterName:            "t1-deeb",
				NodeName:               nodeName,
				DirWithNodeIDFile:      nodeIDDir,
				ClusterSize:            3,
				GRPCPort:               "10000",
				TransportClientFactory: tcf,
				Logger:                 logger,
			}
		},
	)

	return fx.Options(provides, fx.Invoke(invokeFunc))
}
