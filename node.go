package myriad

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"time"

	"github.com/ChrisRx/jolt"
	"github.com/hashicorp/memberlist"
	hraft "github.com/hashicorp/raft"
	"github.com/pkg/errors"

	"github.com/ChrisRx/myriad/gossip"
	"github.com/ChrisRx/myriad/netutil"
	"github.com/ChrisRx/myriad/raft"
	"github.com/ChrisRx/myriad/rpc"
)

type Node struct {
	config *Config

	*gossip.Gossip

	*raft.Raft

	*rpc.Server

	ctx    context.Context
	cancel context.CancelFunc

	logger *jolt.Logger
}

func New(c *Config) (*Node, error) {
	if c.Logger == nil {
		c.Logger = jolt.DefaultLogger()
	}
	if c.Debug {
		gossip.SetDebug(os.Stderr)
		raft.SetDebug(os.Stderr)
	}
	host, port := netutil.ParseAddr(c.Addr)
	if host == "" {
		host = "localhost"
	}
	config := memberlist.DefaultLANConfig()
	config.Name = c.Addr
	config.BindAddr = host
	config.BindPort = port
	config.AdvertisePort = port
	trans, err := gossip.NewGossipLayer(c.Addr)
	if err != nil {
		return nil, err
	}
	config.Transport = trans
	n := &Node{
		config: c,
		logger: c.Logger,
	}
	n.ctx, n.cancel = context.WithCancel(context.Background())
	n.Server, err = rpc.NewServer(&rpc.Config{
		Addr:   c.Addr,
		Logger: c.Logger,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	tcpAddr, err := net.ResolveTCPAddr("tcp", c.Addr)
	if err != nil {
		return nil, err
	}
	raftLayer := raft.NewRaftLayer(tcpAddr)
	raftConfig := &raft.Config{
		Debug:               c.Debug,
		Dir:                 c.Dir,
		Addr:                c.Addr,
		RetainSnapshotCount: c.RetainSnapshotCount,
		EnableSingle:        c.EnableSingle,
		RaftTimeout:         c.RaftTimeout,
		StreamLayer:         raftLayer,
		InitialPeers:        c.InitialPeers,
	}
	n.Raft, err = raft.NewRaft(raftConfig)
	if err != nil {
		return nil, err
	}
	err = n.Server.Register(n.Raft.RPC)
	n.Gossip, err = gossip.NewGossip(config)
	if err != nil {
		return nil, err
	}
	err = n.Gossip.SetLocalNode(&gossip.Member{
		ID:   n.Raft.LocalID(),
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, err
	}
	n.Server.AddNetworkLayer(trans)
	n.Server.AddNetworkLayer(raftLayer)
	go n.gossipJoin()
	go n.leaderLoop()
	return n, nil
}

func (n *Node) gossipJoin() {
	for {
		_, err := n.Gossip.Join(n.config.InitialPeers)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			n.logger.Print(err)
			continue
		}
		break
	}
}

func (n *Node) leaderLoop() {
	for {
		if !n.IsLeader() {
			time.Sleep(10 * time.Second)
			continue
		}
		select {
		case ev := <-n.Gossip.Events():
			peers := n.Config().Servers
			var numVoter, numNonvoter, numStaging int
			for _, p := range peers {
				switch p.Suffrage {
				case hraft.Voter:
					numVoter++
				case hraft.Nonvoter:
					numNonvoter++
				case hraft.Staging:
					numStaging++
				}
			}
			switch ev.Event {
			case memberlist.NodeJoin:
				var meta gossip.NodeMeta
				if err := json.Unmarshal(ev.Node.Meta, &meta); err != nil {
					n.logger.Print(err)
					continue
				}
				if numVoter < 5 {
					f := n.Raft.AddVoter(hraft.ServerID(meta.LocalNode.ID), hraft.ServerAddress(meta.LocalNode.Address()), 0, 0)
					if f.Error() != nil {
						n.logger.Print(f.Error())
						continue
					}
					n.logger.Print("node %#v joined as voter", meta.LocalNode.ID)
				} else {
					f := n.Raft.AddNonvoter(hraft.ServerID(meta.LocalNode.ID), hraft.ServerAddress(meta.LocalNode.Address()), 0, 0)
					if f.Error() != nil {
						n.logger.Print(f.Error())
						continue
					}
					n.logger.Print("node %#v joined as nonvoter", meta.LocalNode.ID)
				}
			case memberlist.NodeLeave:
			case memberlist.NodeUpdate:
			}
		case <-n.ctx.Done():
			return
		}
	}
}
