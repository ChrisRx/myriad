package gossip

import (
	"context"
	"net"
	"time"

	"github.com/ChrisRx/jolt"
	"github.com/hashicorp/memberlist"
	"github.com/pkg/errors"

	"github.com/ChrisRx/myriad/rpc"
)

const (
	udpRecvBufSize = 2 * 1024 * 1024
)

type GossipLayer struct {
	ctx    context.Context
	cancel context.CancelFunc

	udpListener *net.UDPConn

	packetCh chan *memberlist.Packet
	streamCh chan net.Conn

	logger *jolt.Logger
}

func NewGossipLayer(addr string) (*GossipLayer, error) {
	l := &GossipLayer{
		packetCh: make(chan *memberlist.Packet),
		streamCh: make(chan net.Conn),
		logger:   jolt.DefaultLogger(),
	}
	l.ctx, l.cancel = context.WithCancel(context.Background())
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	udpListener, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, errors.Errorf("Failed to start UDP listener on %q port %d: %v", addr, udpAddr.Port, err)
	}
	if err := setUDPRecvBuf(udpListener); err != nil {
		return nil, errors.Errorf("Failed to resize UDP buffer: %v", err)
	}
	l.udpListener = udpListener

	go func() {
		for {
			buf := make([]byte, 0xffff)
			n, addr, err := l.udpListener.ReadFrom(buf)
			ts := time.Now()
			if err != nil {
				l.logger.Print(err)
				return
			}
			if n < 1 {
				l.logger.Print("UDP packet too short (%d bytes) %s", len(buf), addr)
				continue
			}
			l.packetCh <- &memberlist.Packet{
				Buf:       buf[:n],
				From:      addr,
				Timestamp: ts,
			}
		}
	}()
	return l, nil
}

func (l *GossipLayer) Type() rpc.RPCType {
	return rpc.GossipRPC
}

func (l *GossipLayer) Handoff(c net.Conn) error {
	select {
	case l.streamCh <- c:
		return nil
	case <-l.ctx.Done():
		return errors.Errorf("Gossip RPC layer closed")
	}
}

func (l *GossipLayer) StreamCh() <-chan net.Conn {
	return l.streamCh
}

func (l *GossipLayer) PacketCh() <-chan *memberlist.Packet {
	return l.packetCh
}

func (l *GossipLayer) WriteTo(b []byte, addr string) (time.Time, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return time.Time{}, err
	}
	_, err = l.udpListener.WriteTo(b, udpAddr)
	return time.Now(), err
}

func (l *GossipLayer) DialTimeout(addr string, timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", string(addr), timeout)
	if err != nil {
		return nil, err
	}
	_, err = conn.Write([]byte{byte(rpc.GossipRPC)})
	if err != nil {
		conn.Close()
		return nil, err
	}
	return conn, err
}

func (l *GossipLayer) FinalAdvertiseAddr(ip string, port int) (net.IP, int, error) {
	advertiseAddr := l.udpListener.LocalAddr().(*net.UDPAddr).IP
	return advertiseAddr, port, nil
}

func (l *GossipLayer) Shutdown() error {
	if l.cancel != nil {
		l.cancel()
	}
	return nil
}

func setUDPRecvBuf(c *net.UDPConn) error {
	size := udpRecvBufSize
	var err error
	for size > 0 {
		if err = c.SetReadBuffer(size); err == nil {
			return nil
		}
		size = size / 2
	}
	return err
}
