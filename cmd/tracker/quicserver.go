package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/dakshcodez/sagittarius/internal/nat"
	"github.com/dakshcodez/sagittarius/internal/network"
	"github.com/dakshcodez/sagittarius/internal/quicconn"
	"github.com/dakshcodez/sagittarius/internal/trackersrv"
	"github.com/quic-go/quic-go"
)

// serveQUIC runs the tracker's NAT-rendezvous listener: nodes dial in
// once and keep the connection open, opening a fresh stream per request
// (REGISTER, LOOKUP, CONNECT, ...) and accepting pushed streams (PUNCH
// signals) the tracker opens back at them.
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

		go handleConn(server, quicconn.Wrap(qconn, stream), rc)
	}
}

// quicPusher lets the tracker registry proactively deliver a message
// (namely PUNCH) to a peer by opening a new stream on its already-open
// control connection.
type quicPusher struct {
	qconn *quic.Conn
}

func (p *quicPusher) Push(msgType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	stream, err := quicconn.OpenStream(p.qconn)
	if err != nil {
		return err
	}
	defer stream.Close()

	conn := network.NewConn(stream)
	return conn.Send(network.Message{
		Type:     msgType,
		SenderID: "tracker",
		Payload:  data,
	})
}
