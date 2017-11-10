package myriad_test

import (
	"testing"
	"time"

	"github.com/ChrisRx/myriad"
	"github.com/google/go-cmp/cmp"
)

func TestCluster(t *testing.T) {
	addrs := []string{":12345", ":12346", ":12347"}
	nodes := make([]*myriad.Node, 0)
	for i, addr := range addrs {
		n, err := myriad.New(&myriad.Config{
			Addr:         addr,
			EnableSingle: i == 0,
			RaftTimeout:  10 * time.Second,
			InitialPeers: addrs,
		})
		if err != nil {
			t.Error(err)
		}
		nodes = append(nodes, n)
	}

	s := nodes[0]

	for i, addr := range addrs[1:] {
		if err := s.Raft.Join(nodes[i+1].LocalID(), addr); err != nil {
			t.Error(err)
		}
	}

	if err := nodes[1].Set("raftkey", "raftvalue"); err != nil {
		t.Error(err)
	}

	v, err := nodes[2].Get("raftkey")
	if err != nil {
		t.Error(err)
	}
	if diff := cmp.Diff(v, "raftvalue"); diff != "" {
		t.Errorf("(-got +want)\n%s", diff)
	}
}
