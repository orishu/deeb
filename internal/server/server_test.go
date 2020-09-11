package server

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/orishu/deeb/internal/lib"
	nd "github.com/orishu/deeb/internal/node"
	"github.com/orishu/deeb/internal/transport"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	_ "github.com/orishu/deeb/internal/statik"
)

func Test_basic_server_operation(t *testing.T) {
	ports, err := freeport.GetFreePorts(2)
	require.NoError(t, err)
	grpcPort := strconv.Itoa(ports[0])
	gwPort := strconv.Itoa(ports[1])
	invokeFunc := func(lc fx.Lifecycle, s *Server) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				err := s.Start(ctx)
				require.NoError(t, err)
				time.Sleep(3 * time.Second)
				return nil
			},
			OnStop: func(ctx context.Context) error {
				return s.Stop(ctx)
			},
		})
	}

	provides := fx.Provide(
		lib.NewDevelopmentLogger,
		func() ServerParams {
			return ServerParams{
				Addr:        "localhost",
				Port:        grpcPort,
				GatewayPort: gwPort,
			}
		},
		func() nd.NodeParams {
			return nd.NodeParams{
				NodeID:       123,
				AddrPort:     nd.NodeInfo{Addr: "localhost", Port: grpcPort},
				IsNewCluster: true,
			}
		},
		New,
		nd.New,
		transport.NewPeerManager,
		transport.NewTransportManager,
		func() transport.ClientFactory {
			return func(context.Context, string, string) (transport.Client, error) {
				return &transport.GRPCClient{}, nil
			}
		},
	)

	app := fxtest.New(t, provides, fx.Invoke(invokeFunc))
	app.RequireStart()

	ctx := context.Background()
	client, err := transport.NewGRPCClient(ctx, "localhost", grpcPort)
	require.NoError(t, err)
	id, err := client.GetRemoteID(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(123), id)

	app.RequireStop()
}
