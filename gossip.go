package distro

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ChrisRx/jolt"
	"github.com/hashicorp/memberlist"
	"github.com/pkg/errors"
)

type Member struct {
	ID   string `json:"id"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

func (m *Member) Address() string {
	return fmt.Sprintf("%s:%d", m.Host, m.Port)
}
func (m *Member) Name() string   { return m.Address() }
func (m *Member) String() string { return m.Address() }

type NodeMeta struct {
	LocalNode *Member `json:"localnode"`
}

type Gossip struct {
	*memberlist.Memberlist

	config *memberlist.Config
	events chan memberlist.NodeEvent
	mu     sync.Mutex

	logger *jolt.Logger
}

func NewGossip(c *Config) (*Gossip, error) {
	if c.GossipConfig == nil {
		c.GossipConfig = memberlist.DefaultLANConfig()
	}
	m, err := memberlist.Create(c.GossipConfig)
	if err != nil {
		return nil, err
	}
	g := &Gossip{
		Memberlist: m,
		config:     c.GossipConfig,
		events:     make(chan memberlist.NodeEvent, 100),
		logger:     c.Logger,
	}
	c.GossipConfig.Events = &memberlist.ChannelEventDelegate{g.events}
	return g, nil
}

func (g *Gossip) Events() <-chan memberlist.NodeEvent { return g.events }

func (g *Gossip) SetLocalNode(m *Member) error {
	if m.Host == "" {
		return errors.Errorf("invalid Host: %#v", m.Host)
	}
	data, err := json.Marshal(NodeMeta{m})
	if err != nil {
		return err
	}
	g.mu.Lock()
	g.LocalNode().Meta = data
	g.mu.Unlock()
	return nil
}

func (g *Gossip) Join(existing []string) (int, error) {
	peers := make([]string, 0)
	for _, addr := range existing {
		host, port := parseAddr(addr)
		if host == "" {
			host = "localhost"
		}
		peers = append(peers, fmt.Sprintf("%s:%d", host, port))
	}
	return g.Memberlist.Join(peers)
}

func (g *Gossip) Members() []*Member {
	members := make([]*Member, 0)
	for _, m := range g.Memberlist.Members() {
		// A member may be in the memberlist but Meta may be nil if the local
		// metadata has yet to be propagated to this node. In this case, we
		// ignore that member considering it to not ready.
		if m.Meta == nil {
			continue
		}
		var meta NodeMeta
		if err := json.Unmarshal(m.Meta, &meta); err != nil {
			// TODO(chris): debug log err
			continue
		}
		members = append(members, meta.LocalNode)
	}
	return members
}
