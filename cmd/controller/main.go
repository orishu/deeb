package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/etcd-io/etcd/raft"
	"github.com/orishu/deeb/internal/backend/mysql"
	"github.com/orishu/deeb/internal/backend/sqlite"
	"github.com/orishu/deeb/internal/bootstrap"
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
	doBootstrap := flag.Bool("bootstrap", false, "Derive node ID, peers, isNewCluster from running environment")
	isNewCluster := flag.Bool("n", false, "Start up a new cluster")
	host := flag.String("host", "localhost", "The address to bind to")
	gRPCPort := flag.String("port", "10000", "The gRPC server port")
	gatewayPort := flag.String("gwport", "11000", "The gRPC-Gateway server port")
	nodeID := flag.Uint64("id", 1, "The node ID")
	peerStr := flag.String("peers", "", "Comma-separated list of addr:port potential raft peers")
	backend := flag.String("backend", "mysql", "Backend type: 'mysql' or 'sqlite'")
	sqliteDBDir := flag.String("sqlitedir", "./db", "directory for the sqlite files")
	sshPrivateKeyFile := flag.String("ssh_priv_key", "id_rsa", "Path to SSH private key file used by MySQL backend")
	retention := flag.Uint64("retention", 1000, "Number of Raft entries to retain")

	flag.Parse()

	var backendProvides fx.Option
	if *backend == "sqlite" {
		_ = os.Mkdir(*sqliteDBDir, 0755)
		backendProvides = fx.Provide(
			func() sqlite.Params {
				return sqlite.Params{
					DBDir:           *sqliteDBDir,
					EntriesToRetain: *retention,
				}
			},
			sqlite.New,
		)
	}
	if *backend == "mysql" {
		sshPrivateKey, err := ioutil.ReadFile(*sshPrivateKeyFile)
		if err != nil {
			panic(err)
		}
		backendProvides = fx.Provide(
			func() mysql.Params {
				return mysql.Params{
					Addr:            "localhost",
					MysqlPort:       3306,
					SSHPort:         22,
					PrivateKey:      sshPrivateKey,
					EntriesToRetain: *retention,
				}
			},
			mysql.New,
		)
	}

	var bootstrapProvide fx.Option
	if *doBootstrap {
		bootstrapProvide = fx.Provide(
			func(ctx context.Context, p bootstrap.Params) bootstrap.BootstrapInfo {
				bsi, err := bootstrap.New(ctx, p)
				if err != nil {
					panic(err)
				}
				return bsi
			},
			func() bootstrap.GRPCPortType { return bootstrap.GRPCPortType(*gRPCPort) },
			bootstrap.NewEnvParams,
		)
	} else {
		bootstrapProvide = fx.Provide(func() bootstrap.BootstrapInfo {
			return bootstrap.BootstrapInfo{
				NodeID:       *nodeID,
				IsNewCluster: *isNewCluster,
				NodeName:     *host,
				Peers:        parsePeers(*peerStr),
			}
		})
	}

	provides := fx.Provide(
		func(bsi bootstrap.BootstrapInfo) nd.NodeParams {
			return nd.NodeParams{
				NodeID:         bsi.NodeID,
				AddrPort:       nd.NodeInfo{Addr: bsi.NodeName, Port: *gRPCPort},
				IsNewCluster:   bsi.IsNewCluster,
				PotentialPeers: bsi.Peers,
			}
		},
		func() server.ServerParams {
			return server.ServerParams{
				Addr:        *host,
				Port:        *gRPCPort,
				GatewayPort: *gatewayPort,
			}
		},
		nd.New,
		server.New,
		transport.NewPeerManager,
		transport.NewTransportManager,
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

	app := fx.New(provides, backendProvides, bootstrapProvide, fx.Invoke(invoke))
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
