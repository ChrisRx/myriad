package raft

import (
	"context"
	"net"
	"time"

	"github.com/hashicorp/raft"
	"github.com/pkg/errors"

	"github.com/ChrisRx/myriad/rpc"
)

type RaftLayer struct {
	addr net.Addr

	connCh chan net.Conn

	ctx    context.Context
	cancel context.CancelFunc
}

func NewRaftLayer(addr net.Addr) *RaftLayer {
	l := &RaftLayer{
		addr:   addr,
		connCh: make(chan net.Conn),
	}
	l.ctx, l.cancel = context.WithCancel(context.Background())
	return l
}

func (l *RaftLayer) Type() rpc.RPCType {
	return rpc.RaftRPC
}

func (l *RaftLayer) Handoff(c net.Conn) error {
	select {
	case l.connCh <- c:
		return nil
	case <-l.ctx.Done():
		return errors.Errorf("Raft RPC layer closed")
	}
}

func (l *RaftLayer) Accept() (net.Conn, error) {
	select {
	case conn := <-l.connCh:
		return conn, nil
	case <-l.ctx.Done():
		return nil, errors.Errorf("Raft RPC layer closed")
	}
}

func (l *RaftLayer) Close() error {
	if l.cancel != nil {
		l.cancel()
	}
	return nil
}

func (l *RaftLayer) Addr() net.Addr { return l.addr }

func (l *RaftLayer) Dial(address raft.ServerAddress, timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", string(address), timeout)
	if err != nil {
		return nil, err
	}
	_, err = conn.Write([]byte{byte(rpc.RaftRPC)})
	if err != nil {
		conn.Close()
		return nil, err
	}
	return conn, err
}
