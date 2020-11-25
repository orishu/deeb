package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/etcd-io/etcd/raft"
	"github.com/orishu/deeb/internal/backend/sqlite"
	"github.com/orishu/deeb/internal/lib"
	nd "github.com/orishu/deeb/internal/node"
	"github.com/orishu/deeb/internal/server"
	"github.com/orishu/deeb/internal/transport"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc/grpclog"

	_ "github.com/orishu/deeb/internal/statik"
)

func main() {
	isNewCluster := flag.Bool("n", false, "Start up a new cluster")
	host := flag.String("host", "localhost", "The address to bind to")
	gRPCPort := flag.String("port", "10000", "The gRPC server port")
	gatewayPort := flag.String("gwport", "11000", "The gRPC-Gateway server port")
	nodeID := flag.Uint64("id", 1, "The node ID")
	peerStr := flag.String("peers", "", "Comma-separated list of addr:port potential raft peers")
	dbdir := flag.String("sqlitedir", "./db", "directory for the sqlite files")

	_ = os.Mkdir(*dbdir, 0755)

	flag.Parse()
	peers := parsePeers(*peerStr)

	provides := fx.Provide(
		func() nd.NodeParams {
			return nd.NodeParams{
				NodeID:         *nodeID,
				AddrPort:       nd.NodeInfo{Addr: *host, Port: *gRPCPort},
				IsNewCluster:   *isNewCluster,
				PotentialPeers: peers,
			}
		},
		func() server.ServerParams {
			return server.ServerParams{
				Addr:        *host,
				Port:        *gRPCPort,
				GatewayPort: *gatewayPort,
			}
		},
		func() sqlite.Params {
			return sqlite.Params{
				DBDir:           *dbdir,
				EntriesToRetain: 5,
			}
		},
		nd.New,
		server.New,
		transport.NewPeerManager,
		transport.NewTransportManager,
		sqlite.New,
		func() transport.ClientFactory { return transport.NewGRPCClient },
		lib.NewDevelopmentLogger,
		lib.NewLoggerAdapter,
	)

	invoke := func(
		lc fx.Lifecycle,
		srv *server.Server,
		logger *zap.SugaredLogger,
		loggerAdapter lib.LoggerAdapter,
	) {
		raft.SetLogger(&loggerAdapter)
		grpclog.SetLoggerV2(&loggerAdapter)
		logger.Info("starting Fx app")
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				logger.Info("OnStart")
				return srv.Start(ctx)
			},
			OnStop: func(ctx context.Context) error {
				logger.Info("OnStop")
				defer logger.Sync()
				return srv.Stop(ctx)
			},
		})
	}

	app := fx.New(provides, fx.Invoke(invoke))
	ctx := context.Background()
	err := app.Start(ctx)
	if err != nil {
		panic(err)
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	app.Stop(ctx)
}

func parsePeers(peers string) []nd.NodeInfo {
	res := make([]nd.NodeInfo, 0)
	for _, p := range strings.Split(peers, ",") {
		if p == "" {
			continue
		}
		addrport := strings.Split(p, ":")
		if len(addrport) != 2 {
			panic(fmt.Sprintf("malformed peer: %s", p))
		}
		res = append(res, nd.NodeInfo{Addr: addrport[0], Port: addrport[1]})
	}
	return res
}
