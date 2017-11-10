package raft

import (
	"time"

	"github.com/ChrisRx/jolt"
	"github.com/hashicorp/raft"
)

type Config struct {
	Debug bool

	Dir string

	Addr string

	RetainSnapshotCount int

	EnableSingle bool

	RaftTimeout time.Duration

	StreamLayer raft.StreamLayer

	InitialPeers []string

	Logger *jolt.Logger
}
