package distro

import (
	"encoding/json"
	"os"
	"time"

	"github.com/ChrisRx/jolt"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"github.com/pkg/errors"
)

type Distro struct {
	config *Config

	*Gossip

	*Raft

	*Server

	logger *jolt.Logger
}

func New(c *Config) (*Distro, error) {
	if c.Logger == nil {
		c.Logger = jolt.DefaultLogger()
	}
	if c.Debug {
		SetDebug(os.Stderr)
	}
	host, port := parseAddr(c.Addr)
	if host == "" {
		host = "localhost"
	}
	c.GossipConfig = memberlist.DefaultLANConfig()
	c.GossipConfig.Name = c.Addr
	c.GossipConfig.Logger = logger
	c.GossipConfig.BindAddr = host
	c.GossipConfig.BindPort = port
	c.GossipConfig.AdvertisePort = port
	trans, err := NewGossipLayer(c.Addr)
	if err != nil {
		return nil, err
	}
	c.GossipConfig.Transport = trans
	d := &Distro{
		config: c,
		logger: c.Logger,
	}
	d.Raft, err = NewRaft(c)
	if err != nil {
		return nil, err
	}
	d.Server, err = NewServer(c)
	if err != nil {
		return nil, err
	}
	err = d.Server.Register(d.Raft.RPC)
	if err != nil {
		return nil, err
	}
	d.Gossip, err = NewGossip(c)
	if err != nil {
		return nil, err
	}
	err = d.Gossip.SetLocalNode(&Member{
		ID:   d.Raft.LocalID(),
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	go d.gossipJoin()
	go d.leaderLoop()
	return d, nil
}

func (d *Distro) gossipJoin() {
	for {
		_, err := d.Gossip.Join(d.config.InitialPeers)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			d.logger.Print(err)
			continue
		}
		break
	}
}

func (d *Distro) leaderLoop() {
	for {
		if !d.IsLeader() {
			time.Sleep(10 * time.Second)
			continue
		}
		select {
		case ev := <-d.Gossip.Events():
			//members := d.Gossip.Members()
			//numMembers := len(members)
			peers := d.Config().Servers
			//numPeers := len(peers)
			var numVoter, numNonvoter, numStaging int
			for _, p := range peers {
				switch p.Suffrage {
				case raft.Voter:
					numVoter++
				case raft.Nonvoter:
					numNonvoter++
				case raft.Staging:
					numStaging++
				}
			}
			switch ev.Event {
			case memberlist.NodeJoin:
				var meta NodeMeta
				if err := json.Unmarshal(ev.Node.Meta, &meta); err != nil {
					d.logger.Print(err)
					continue
				}
				if numVoter < 5 {
					// AddVoter
					f := d.Raft.AddVoter(raft.ServerID(meta.LocalNode.ID), raft.ServerAddress(meta.LocalNode.Address()), 0, 0)
					if f.Error() != nil {
						d.logger.Print(f.Error())
						continue
					}
					d.logger.Print("node %#v joined as voter", meta.LocalNode.ID)
				} else {
					// AddNonVoter
					f := d.Raft.AddNonvoter(raft.ServerID(meta.LocalNode.ID), raft.ServerAddress(meta.LocalNode.Address()), 0, 0)
					if f.Error() != nil {
						d.logger.Print(f.Error())
						continue
					}
					d.logger.Print("node %#v joined as nonvoter", meta.LocalNode.ID)
				}
			case memberlist.NodeLeave:
			case memberlist.NodeUpdate:
			}
		case <-d.ctx.Done():
			return
		}
	}
}
