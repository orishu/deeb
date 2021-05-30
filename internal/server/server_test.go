package server

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	pb "github.com/orishu/deeb/api"
	"github.com/orishu/deeb/internal/backend/sqlite"
	"github.com/orishu/deeb/internal/lib"
	nd "github.com/orishu/deeb/internal/node"
	"github.com/orishu/deeb/internal/transport"
	"github.com/phayes/freeport"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"google.golang.org/grpc"

	_ "github.com/orishu/deeb/internal/statik"
)

func Test_basic_server_operation(t *testing.T) {
	dbdir, err := ioutil.TempDir(".", fmt.Sprintf("%s-*", t.Name()))
	require.NoError(t, err)
	defer os.RemoveAll(dbdir)

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
		func() sqlite.Params {
			return sqlite.Params{
				DBDir:           dbdir,
				EntriesToRetain: 5,
			}
		},
		New,
		nd.New,
		sqlite.New,
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
	id, err = client.GetHighestID(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(123), id)

	svcClient, closer, err := makeServiceClient(ctx, grpcPort)
	defer closer()
	require.NoError(t, err)

	sql := `CREATE TABLE table1 (col1 INTEGER, col2 TEXT)`
	_, err = svcClient.ExecuteSQL(ctx, &pb.ExecuteSQLRequest{Sql: sql})
	require.NoError(t, err)
	sql = `INSERT INTO table1 (col1, col2) VALUES (10, "ten"), (20, "twenty")`
	_, err = svcClient.ExecuteSQL(ctx, &pb.ExecuteSQLRequest{Sql: sql})
	require.NoError(t, err)
	sql = `SELECT * FROM table1`
	sqlRecv, err := svcClient.QuerySQL(ctx, &pb.QuerySQLRequest{Sql: sql})
	require.NoError(t, err)
	type record struct {
		col1 int
		col2 string
	}
	result := make([]record, 0, 2)
	for {
		res, err := sqlRecv.Recv()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		require.NotNil(t, res)
		result = append(result, record{
			col1: int(res.Row.Cells[0].GetI64()),
			col2: res.Row.Cells[1].GetStr(),
		})
	}
	fmt.Printf("Result: %+v\n", result)
	require.Equal(t, 2, len(result))
	require.Equal(t, 10, result[0].col1)
	require.Equal(t, "ten", result[0].col2)
	require.Equal(t, 20, result[1].col1)
	require.Equal(t, "twenty", result[1].col2)

	prog, err := client.Progress(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(123), prog.Id)
	require.Equal(t, "StateLeader", prog.State)
	require.Equal(t, uint64(4), prog.Applied)
	thisProg, ok := prog.ProgressMap[123]
	require.True(t, ok)
	require.NotNil(t, thisProg)
	require.Equal(t, uint64(4), thisProg.Match)

	err = client.CheckHealth(ctx)
	require.NoError(t, err)

	app.RequireStop()
}

func makeServiceClient(ctx context.Context, grpcPort string) (pb.ControlServiceClient, func(), error) {
	addr := "localhost"
	conn, err := grpc.DialContext(ctx, net.JoinHostPort(addr, grpcPort),
		grpc.WithInsecure(),
		//		grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(insecure.CertPool, "")),
	)
	if err != nil {
		return nil, func() {}, errors.Wrapf(err, "grpc dial error %s:%s", addr, grpcPort)
	}
	return pb.NewControlServiceClient(conn), func() { conn.Close() }, nil
}
