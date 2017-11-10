package gossip

import (
	"github.com/ChrisRx/jolt"
	"github.com/hashicorp/memberlist"
)

type Config struct {
	Addr string

	InitialPeers []string

	GossipConfig *memberlist.Config

	Logger *jolt.Logger
}
