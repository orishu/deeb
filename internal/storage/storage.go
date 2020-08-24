package storage

import (
	"fmt"
	"github.com/etcd-io/etcd/raft"
)

// Foo is ...
func Foo() {
	c := raft.Config{}
	fmt.Printf("raft config: %+v\n", c)
}
