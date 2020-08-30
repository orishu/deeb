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

// TODO: integrate this with main()
func main1() {
	fmt.Println("Controller")
	storage.Foo()
	n := node.New()
	go func() {
		n.Start()
	}()
	n.Stop()
}

func main() {
	flag.Parse()
	ctx := context.Background()

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
	}()
	srv.Start(ctx)
}
