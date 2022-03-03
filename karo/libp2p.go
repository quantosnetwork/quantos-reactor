package karo

import (
	"context"
	"sync"
)

/*
	KARO is the name of the quantos library for the peer-to-peer network
*/

type Node struct {
	reactor *Reactor
}

func CreateNewNode() {
	n := &Node{}
	n.reactor = new(Reactor)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Done()
	go func() {
		n.reactor.Initialize(n, ctx, cancel)
	}()
	wg.Wait()

}
