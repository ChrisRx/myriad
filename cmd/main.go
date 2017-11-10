package main

import (
	"fmt"
	"log"
	"time"

	"github.com/ChrisRx/myriad"
)

func main() {
	addrs := []string{":12345", ":12346", ":12347"}
	nodes := make([]*myriad.Node, 0)
	for i, addr := range addrs {
		n, err := myriad.New(&myriad.Config{
			Dir:                 "data",
			Addr:                addr,
			EnableSingle:        i == 0,
			RaftTimeout:         10 * time.Second,
			RetainSnapshotCount: 2,
			InitialPeers:        addrs,
		})
		if err != nil {
			log.Fatal(err)
		}
		nodes = append(nodes, n)
	}

	addrs2 := []string{":12348", ":12349", ":12350"}
	for i, addr := range addrs2 {
		n, err := myriad.New(&myriad.Config{
			Dir:                 "data",
			Addr:                addr,
			EnableSingle:        i == 0,
			RaftTimeout:         10 * time.Second,
			RetainSnapshotCount: 2,
			InitialPeers:        addrs,
		})
		if err != nil {
			log.Fatal(err)
		}
		nodes = append(nodes, n)
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
