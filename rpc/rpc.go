package rpc

import (
	"context"
	"io"
	"net"
	"net/rpc"

	"github.com/ChrisRx/jolt"
)

type RPCType byte

const (
	CommandRPC RPCType = 0x01
	RaftRPC            = 0x02
	GossipRPC          = 0x03
)

type NetworkLayer interface {
	Type() RPCType
	Handoff(net.Conn) error
}

type Config struct {
	Addr string

	Logger *jolt.Logger
}

type Server struct {
	net.Listener
	*rpc.Server

	ctx    context.Context
	cancel context.CancelFunc

	layers map[RPCType]NetworkLayer

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
		Listener: list,
		layers:   make(map[RPCType]NetworkLayer),
		logger:   c.Logger,
		Server:   rpc.NewServer(),
	}
	s.AddNetworkLayer(s)
	s.ctx, s.cancel = context.WithCancel(context.Background())
	go s.listen()
	return s, nil
}

func (s *Server) Type() RPCType {
	return CommandRPC
}

func (s *Server) AddNetworkLayer(layer NetworkLayer) {
	s.layers[layer.Type()] = layer
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
	if handler, ok := s.layers[RPCType(buf[0])]; ok {
		if err := handler.Handoff(conn); err != nil {
			s.logger.Print(err)
			return
		}
	} else {
		s.logger.Print("unrecognized RPC byte: %v", buf[0])
		conn.Close()
	}
}

func (s *Server) Handoff(conn net.Conn) error {
	defer conn.Close()
	s.Server.ServeConn(conn)
	return nil
}

func (s *Server) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}
