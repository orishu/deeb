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
	"github.com/orishu/deeb/internal/lib"
	nd "github.com/orishu/deeb/internal/node"
	"github.com/orishu/deeb/internal/server"
	"github.com/orishu/deeb/internal/storage"
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

	flag.Parse()
	peers := parsePeers(*peerStr)

	opts := fx.Options(
		fx.Provide(func() nd.NodeParams {
			return nd.NodeParams{
				NodeID:         *nodeID,
				AddrPort:       nd.NodeInfo{Addr: *host, Port: *gRPCPort},
				IsNewCluster:   *isNewCluster,
				PotentialPeers: peers,
			}
		}),
		fx.Provide(func() server.ServerParams {
			return server.ServerParams{
				Addr:        *host,
				Port:        *gRPCPort,
				GatewayPort: *gatewayPort,
			}
		}),
		fx.Provide(nd.New),
		fx.Provide(server.New),
		fx.Provide(makeLogger),
		fx.Provide(lib.NewLoggerAdapter),
		fx.Invoke(func(
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
		}),
	)

	// TODO: this is a placeholder
	storage.Foo()

	app := fx.New(opts)
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

func makeLogger() *zap.SugaredLogger {
	l, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	return l.Sugar()
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
