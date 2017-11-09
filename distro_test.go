package distro_test

import (
	"net"
	"testing"
	"time"

	"github.com/ChrisRx/distro"
	"github.com/google/go-cmp/cmp"
)

func TestDistro(t *testing.T) {
	addrs := []string{":12345", ":12346", ":12347"}
	nodes := make([]*distro.Distro, 0)
	for i, addr := range addrs {
		tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			t.Error(err)
		}
		s, err := distro.New(&distro.Config{
			Addr:         addr,
			EnableSingle: i == 0,
			RaftTimeout:  10 * time.Second,
			StreamLayer:  distro.NewRaftLayer(tcpAddr),
			InitialPeers: addrs,
		})
		if err != nil {
			t.Error(err)
		}
		nodes = append(nodes, s)
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
