package karo

import (
	"github.com/libp2p/go-libp2p-core/event"
	"github.com/quantosnetwork/quantos-reactor/karo/handler"
)

type Event struct {
	Type string
}

type PubSubEvent struct {
	SubOptions handler.EventSubOptions
	PubOptions event.EmitterOpt
}

func EventSubOptions(e interface{}) error {
	return nil
}
