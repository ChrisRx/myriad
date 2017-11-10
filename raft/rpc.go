package raft

import (
	"net"
	"net/rpc"

	mrpc "github.com/ChrisRx/myriad/rpc"
)

type RaftRPC struct {
	raft *Raft
}

func (r *RaftRPC) newClient() (*rpc.Client, error) {
	conn, err := net.Dial("tcp", string(r.raft.Leader()))
	if err != nil {
		return nil, err
	}
	conn.Write([]byte{byte(mrpc.CommandRPC)})
	client := rpc.NewClient(conn)
	return client, nil
}

func (r *RaftRPC) Get(c *Command, resp *string) error {
	if r.raft.IsLeader() {
		v, err := r.raft.Get(c.Key)
		if err != nil {
			return err
		}
		*resp = v
		return nil
	}
	client, err := r.newClient()
	if err != nil {
		return err
	}
	defer client.Close()
	err = client.Call("RaftRPC.Get", c, resp)
	if err != nil {
		return err
	}
	return nil
}

func (r *RaftRPC) Set(c *Command, resp *string) error {
	if r.raft.IsLeader() {
		if err := r.raft.Set(c.Key, c.Value); err != nil {
			return err
		}
		return nil
	}
	client, err := r.newClient()
	if err != nil {
		return err
	}
	defer client.Close()
	err = client.Call("RaftRPC.Set", c, resp)
	if err != nil {
		return err
	}
	return nil
}

func (r *RaftRPC) Delete(c *Command, resp *string) error {
	if r.raft.IsLeader() {
		if err := r.raft.Delete(c.Key); err != nil {
			return err
		}
		return nil
	}
	client, err := r.newClient()
	if err != nil {
		return err
	}
	defer client.Close()
	err = client.Call("RaftRPC.Delete", c, resp)
	if err != nil {
		return err
	}
	return nil
}
