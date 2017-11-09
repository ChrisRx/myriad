package distro

import (
	"time"

	"github.com/ChrisRx/jolt"
	"github.com/hashicorp/memberlist"
)

type Config struct {
	Debug bool

	Dir string

	Addr string

	ID string

	RetainSnapshotCount int

	EnableSingle bool

	RaftTimeout time.Duration

	StreamLayer *RaftLayer

	Store *Store

	InitialPeers []string

	GossipConfig *memberlist.Config

	Logger *jolt.Logger
}
