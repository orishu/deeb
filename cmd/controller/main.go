package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/etcd-io/etcd/raft"
	"github.com/orishu/deeb/internal/node"
	"github.com/orishu/deeb/internal/server"
	"github.com/orishu/deeb/internal/storage"

	_ "github.com/orishu/deeb/internal/statik"
)

func main() {
	isNewCluster := flag.Bool("n", false, "Start up a new cluster")
	host := flag.String("host", "localhost", "The address to bind to")
	gRPCPort := flag.String("port", "10000", "The gRPC server port")
	gatewayPort := flag.String("gwport", "11000", "The gRPC-Gateway server port")
	nodeID := flag.Uint64("id", 1, "The node ID")
	peerStr := flag.String("peers", "", "Comma-separated list of addr:port potential raft peers")

	raftLogger := &raft.DefaultLogger{Logger: log.New(os.Stderr, "raft", log.LstdFlags)}
	//raftLogger := &raft.DefaultLogger{Logger: log.New(ioutil.Discard, "", 0)}
	raftLogger.EnableDebug()
	raft.SetLogger(raftLogger)

	flag.Parse()
	peers := parsePeers(*peerStr)

	ctx := context.Background()
	n := node.New(*nodeID, node.NodeInfo{Addr: *host, Port: *gRPCPort})
	storage.Foo()

	srv := server.New(
		n,
		server.ServerParams{Addr: *host, Port: *gRPCPort, GatewayPort: *gatewayPort},
	)
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("Shutting down")
		err := srv.Stop(ctx)
		if err != nil {
			fmt.Printf("stopping error: %+v\n", err)
			os.Exit(1)
		}
		n.Stop()
	}()
	go func() {
		n.Start(ctx, *isNewCluster, peers)
	}()
	srv.Start(ctx)
}

func parsePeers(peers string) []node.NodeInfo {
	res := make([]node.NodeInfo, 0)
	for _, p := range strings.Split(peers, ",") {
		if p == "" {
			continue
		}
		addrport := strings.Split(p, ":")
		if len(addrport) != 2 {
			panic(fmt.Sprintf("malformed peer: %s", p))
		}
		res = append(res, node.NodeInfo{Addr: addrport[0], Port: addrport[1]})
	}
	return res
}
