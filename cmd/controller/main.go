package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/orishu/deeb/internal/node"
	"github.com/orishu/deeb/internal/server"
	"github.com/orishu/deeb/internal/storage"

	_ "github.com/orishu/deeb/internal/statik"
)

func main() {
	flag.Parse()
	ctx := context.Background()
	n := node.New()
	storage.Foo()

	srv := server.New()
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
		n.Start()
	}()
	srv.Start(ctx)
}
