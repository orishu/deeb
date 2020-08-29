package main

import (
	"fmt"
	"time"

	"github.com/orishu/deeb/internal/node"
	"github.com/orishu/deeb/internal/storage"
)

func main() {
	fmt.Println("Controller")
	storage.Foo()
	n := node.New()
	go func() {
		n.Start()
	}()
	time.Sleep(3 * time.Second)
	n.Stop()
}
