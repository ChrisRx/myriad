package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/ChrisRx/distro"
)

func main() {
	addrs := []string{":12345", ":12346", ":12347"}
	nodes := make([]*distro.Distro, 0)
	for i, addr := range addrs {
		tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			log.Fatal(err)
		}
		s, err := distro.New(&distro.Config{
			Dir:                 "data",
			Addr:                addr,
			EnableSingle:        i == 0,
			RaftTimeout:         10 * time.Second,
			RetainSnapshotCount: 2,
			StreamLayer:         distro.NewRaftLayer(tcpAddr),
			InitialPeers:        addrs,
		})
		if err != nil {
			log.Fatal(err)
		}
		nodes = append(nodes, s)
	}

	addrs2 := []string{":12348", ":12349", ":12350"}
	for i, addr := range addrs2 {
		tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			log.Fatal(err)
		}
		s, err := distro.New(&distro.Config{
			Dir:                 "data",
			Addr:                addr,
			EnableSingle:        i == 0,
			RaftTimeout:         10 * time.Second,
			RetainSnapshotCount: 2,
			StreamLayer:         distro.NewRaftLayer(tcpAddr),
			InitialPeers:        addrs,
		})
		if err != nil {
			log.Fatal(err)
		}
		nodes = append(nodes, s)
	}

	s := nodes[0]

	for i, addr := range addrs[1:] {
		if err := s.Raft.Join(nodes[i+1].LocalID(), addr); err != nil {
			log.Fatal(err)
		}
	}

	if err := nodes[1].Set("raftkey", "raftvalue"); err != nil {
		log.Fatal(err)
	}

	v, err := nodes[2].Get("raftkey")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("v = %+v\n", v)

	for {
		time.Sleep(5 * time.Second)
		for _, server := range s.Config().Servers {
			fmt.Printf("server = %+v\n", server)
		}

	}
}
