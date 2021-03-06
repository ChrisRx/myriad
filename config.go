package myriad

import (
	"time"

	"github.com/ChrisRx/jolt"
)

type Config struct {
	Debug bool

	Dir string

	Addr string

	ID string

	RetainSnapshotCount int

	EnableSingle bool

	RaftTimeout time.Duration

	InitialPeers []string

	Logger *jolt.Logger
}
