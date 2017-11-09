package distro

import (
	"context"
	"io"
	"net"
	"net/rpc"

	"github.com/ChrisRx/jolt"
)

type RPCType byte

const (
	rpcCommand RPCType = 0x01
	rpcRaft            = 0x02
	rpcGossip          = 0x03
)

type Server struct {
	net.Listener
	*rpc.Server

	ctx    context.Context
	cancel context.CancelFunc

	gossipLayer *GossipLayer
	raftLayer   *RaftLayer

	store *Store

	logger *jolt.Logger
}

func NewServer(c *Config) (*Server, error) {
	addr, err := net.ResolveTCPAddr("tcp", c.Addr)
	if err != nil {
		return nil, err
	}
	list, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := &Server{
		Listener:    list,
		gossipLayer: c.GossipConfig.Transport.(*GossipLayer),
		raftLayer:   c.StreamLayer,
		logger:      c.Logger,
		store:       c.Store,
		Server:      rpc.NewServer(),
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	go s.listen()
	return s, nil
}

func (s *Server) listen() {
	for {
		conn, err := s.Listener.Accept()
		if err != nil {
			if err == context.Canceled {
				return
			}
			s.logger.Print("failed to accept RPC conn: %v", err)
			continue
		}

		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err != nil {
		if err != io.EOF {
			s.logger.Print("failed to read byte: %v", err)
		}
		conn.Close()
		return
	}
	switch RPCType(buf[0]) {
	case rpcCommand:
		s.handleCommandConn(conn)
	case rpcRaft:
		s.raftLayer.Handoff(conn)
	case rpcGossip:
		s.gossipLayer.Handoff(conn)
	default:
		s.logger.Print("unrecognized RPC byte: %v", buf[0])
		conn.Close()
		return
	}
}

func (s *Server) handleCommandConn(conn net.Conn) {
	defer conn.Close()
	s.Server.ServeConn(conn)
}

func (s *Server) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}
