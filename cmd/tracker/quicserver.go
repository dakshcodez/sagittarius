package main

import (
	"context"
	"encoding/json"
	"log"
	"net"

	"github.com/dakshcodez/sagittarius/internal/nat"
	"github.com/dakshcodez/sagittarius/internal/network"
	"github.com/dakshcodez/sagittarius/internal/quicconn"
	"github.com/dakshcodez/sagittarius/internal/trackersrv"
	"github.com/quic-go/quic-go"
)

// serveQUIC runs the tracker's NAT-rendezvous listener: nodes dial in
// once and keep the connection open, opening a fresh stream per request
// (REGISTER, LOOKUP, CONNECT, RELAY, ...) and accepting pushed streams
// (PUNCH signals, relayed data connections) the tracker opens back at
// them.
func serveQUIC(server *trackersrv.Server, udpAddr string) {
	socket, err := nat.Open(udpAddr)
	if err != nil {
		log.Fatalf("tracker: quic listen on %s failed: %v", udpAddr, err)
	}
	defer socket.Close()

	log.Printf("tracker: QUIC/NAT rendezvous listening on %s", udpAddr)

	ctx := context.Background()

	for {
		qconn, err := socket.Accept(ctx)
		if err != nil {
			log.Printf("tracker: quic accept failed: %v", err)
			continue
		}

		go serveQUICConn(server, qconn)
	}
}

// serveQUICConn handles every stream a single peer's connection opens for
// the lifetime of that connection.
func serveQUICConn(server *trackersrv.Server, qconn *quic.Conn) {
	rc := trackersrv.RequestContext{
		ReflexiveAddr: qconn.RemoteAddr().String(),
		Pusher:        &quicPusher{qconn: qconn},
	}

	ctx := qconn.Context()

	for {
		stream, err := qconn.AcceptStream(ctx)
		if err != nil {
			return
		}

		go handleQUICStream(server, quicconn.Wrap(qconn, stream), rc)
	}
}

// handleQUICStream peeks at the first message on a newly opened stream:
// a RELAY switches the stream into raw byte-splicing mode (see relay.go)
// instead of the normal JSON request/reply dispatch.
func handleQUICStream(server *trackersrv.Server, raw net.Conn, rc trackersrv.RequestContext) {
	conn := network.NewConn(raw)

	msg, err := conn.Receive()
	if err != nil {
		raw.Close()
		return
	}

	if msg.Type == trackersrv.MsgRelay {
		handleRelay(server.Registry, raw, msg)
		return
	}

	if err := server.HandleMessage(msg, conn, rc); err != nil {
		log.Printf("tracker: error handling %s from %s: %v", msg.Type, msg.SenderID, err)
	}

	handleConn(server, raw, rc)
}

// quicPusher lets the tracker registry proactively deliver a message to
// a peer, or open a raw stream to it, by acting on its already-open
// control connection.
type quicPusher struct {
	qconn *quic.Conn
}

func (p *quicPusher) Push(msgType string, payload any) error {
	stream, err := p.OpenStream(msgType, payload)
	if err != nil {
		return err
	}
	return stream.Close()
}

func (p *quicPusher) OpenStream(msgType string, payload any) (net.Conn, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	stream, err := quicconn.OpenStream(p.qconn)
	if err != nil {
		return nil, err
	}

	conn := network.NewConn(stream)
	if err := conn.Send(network.Message{
		Type:     msgType,
		SenderID: "tracker",
		Payload:  data,
	}); err != nil {
		stream.Close()
		return nil, err
	}

	return stream, nil
}
