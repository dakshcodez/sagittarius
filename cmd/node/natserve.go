package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dakshcodez/sagittarius/internal/nat"
	"github.com/dakshcodez/sagittarius/internal/network"
	"github.com/dakshcodez/sagittarius/internal/quicconn"
	"github.com/dakshcodez/sagittarius/internal/transfer"
	"github.com/quic-go/quic-go"
)

// natAcceptLoop mirrors acceptLoop (serve.go) for the QUIC/NAT socket:
// every inbound QUIC connection may carry multiple streams over its
// lifetime (one per request our peer makes of us), each handled the same
// way a plain TCP connection would be.
func natAcceptLoop(socket *nat.Socket, tm *transfer.TransferManager) {
	ctx := context.Background()

	for {
		qconn, err := socket.Accept(ctx)
		if err != nil {
			log.Printf("node: nat accept failed: %v", err)
			continue
		}

		go natAcceptStreams(qconn, tm)
	}
}

func natAcceptStreams(qconn *quic.Conn, tm *transfer.TransferManager) {
	ctx := qconn.Context()

	for {
		stream, err := qconn.AcceptStream(ctx)
		if err != nil {
			return
		}

		go handleIncoming(quicconn.Wrap(qconn, stream), tm)
	}
}

// natConnectToPeer asks the tracker to broker a connection to
// targetPeerID, races dials against every candidate it returns, and
// performs the initiator handshake on the winning connection. The
// returned bool is true when the winning candidate was a LAN address
// (i.e. no NAT traversal was actually needed) - useful for logging
// direct vs. punched.
func natConnectToPeer(
	client *natClient,
	socket *nat.Socket,
	selfID, targetPeerID string,
	timeout time.Duration,
) (*network.Conn, string, string, error) {

	candidates, err := client.connect(targetPeerID)
	if err != nil {
		return nil, "", "", fmt.Errorf("connect via tracker: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	qconn, winningAddr, err := nat.DialCandidates(ctx, socket, candidates, timeout)
	if err != nil {
		return nil, "", "", fmt.Errorf("punch to %s: %w", targetPeerID, err)
	}

	streamConn, err := quicconn.OpenStream(qconn)
	if err != nil {
		return nil, "", "", fmt.Errorf("open stream to %s: %w", targetPeerID, err)
	}

	conn := network.NewConn(streamConn)

	peerID, err := handshakeAsInitiator(conn, selfID)
	if err != nil {
		streamConn.Close()
		return nil, "", "", fmt.Errorf("handshake with %s: %w", targetPeerID, err)
	}

	return conn, peerID, winningAddr, nil
}
