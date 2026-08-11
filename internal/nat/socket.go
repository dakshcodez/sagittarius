// Package nat implements the mechanics of connecting two peers that may
// each be behind a NAT: gathering address candidates, punching a UDP hole
// via a rendezvous signal, and promoting the punched path to a reliable
// QUIC connection.
//
// The key trick: a single net.PacketConn is wrapped in one quic.Transport
// per node, and that Transport is used for BOTH dialing out (to the
// tracker, and later to peers) and listening for inbound connections.
// quic-go multiplexes by QUIC connection ID rather than by 4-tuple, so
// this is supported on one socket - which is exactly what keeps the NAT
// mapping the tracker observed alive when a later peer-to-peer punch
// reuses the same local port.
package nat

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/quic-go/quic-go"
)

// Socket is one node's UDP socket, QUIC transport, and ambient listener.
type Socket struct {
	udpConn   *net.UDPConn
	transport *quic.Transport
	tlsConfig *tls.Config
	listener  *quic.Listener
}

// Open binds a UDP socket at bindAddr (e.g. ":0" for an OS-assigned port)
// and starts an ambient QUIC listener on it.
func Open(bindAddr string) (*Socket, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", bindAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", bindAddr, err)
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, fmt.Errorf("listen udp %s: %w", bindAddr, err)
	}

	tlsConf, err := GenerateTLSConfig()
	if err != nil {
		udpConn.Close()
		return nil, fmt.Errorf("generate tls config: %w", err)
	}

	transport := &quic.Transport{Conn: udpConn}

	listener, err := transport.Listen(tlsConf, quicConfig())
	if err != nil {
		udpConn.Close()
		return nil, fmt.Errorf("quic listen: %w", err)
	}

	return &Socket{
		udpConn:   udpConn,
		transport: transport,
		tlsConfig: tlsConf,
		listener:  listener,
	}, nil
}

func quicConfig() *quic.Config {
	return &quic.Config{}
}

// LocalPort returns the UDP port this socket is bound to.
func (s *Socket) LocalPort() int {
	return s.udpConn.LocalAddr().(*net.UDPAddr).Port
}

// Dial opens a QUIC connection to addr over this socket.
func (s *Socket) Dial(ctx context.Context, addr string) (*quic.Conn, error) {
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", addr, err)
	}

	return s.transport.Dial(ctx, raddr, s.tlsConfig, quicConfig())
}

// Accept blocks until a peer establishes a QUIC connection to this socket.
func (s *Socket) Accept(ctx context.Context) (*quic.Conn, error) {
	return s.listener.Accept(ctx)
}

// Close shuts down the listener, transport, and underlying UDP socket.
func (s *Socket) Close() error {
	s.listener.Close()
	s.transport.Close()
	return s.udpConn.Close()
}
