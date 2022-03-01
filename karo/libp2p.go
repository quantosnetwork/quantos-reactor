package karo

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

/*
	KARO is the name of the quantos library for the peer-to-peer network
*/

type Node struct {
	reactor *Reactor
}

func CreateNewNode() {
	n := new(Node)
	ctx, cancel := context.WithCancel(context.Background())
	n.reactor = &Reactor{}
	go n.reactor.Initialize(ctx, cancel)
	n.run(cancel)
}

func (n *Node) run(cancel func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	<-c

	fmt.Printf("\rExiting...\n")

	cancel()

	if err := n.reactor.Host.Close(); err != nil {
		panic(err)
	}
	os.Exit(0)
}
