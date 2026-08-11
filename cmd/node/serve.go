package main

import (
	"errors"
	"log"
	"net"

	"github.com/dakshcodez/sagittarius/internal/network"
	"github.com/dakshcodez/sagittarius/internal/transfer"
)

// acceptLoop accepts incoming peer connections and handles each on its own
// goroutine for the life of the connection. It returns once the listener
// is closed.
func acceptLoop(listener net.Listener, tm *transfer.TransferManager) {
	for {
		raw, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			log.Printf("node: accept failed: %v", err)
			continue
		}

		go handleIncoming(raw, tm)
	}
}

// handleIncoming performs the responder side of the handshake (receive
// then reply) and then feeds every subsequent message to the transfer
// manager until the connection closes.
func handleIncoming(raw net.Conn, tm *transfer.TransferManager) {
	defer raw.Close()

	conn := network.NewConn(raw)

	peerID, err := network.ReceiveHandshake(conn)
	if err != nil {
		log.Printf("node: handshake from %s failed: %v", raw.RemoteAddr(), err)
		return
	}

	if err := network.SendHandshake(conn, tm.SelfID()); err != nil {
		log.Printf("node: handshake reply to %s failed: %v", peerID, err)
		return
	}

	log.Printf("node: connected to peer %s (%s)", peerID, raw.RemoteAddr())

	messageLoop(conn, tm, peerID)
}

// dialPeer performs the initiator side of the handshake (send then
// receive) and returns a ready-to-use connection.
func dialPeer(addr string, selfID string) (*network.Conn, string, error) {
	raw, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, "", err
	}

	conn := network.NewConn(raw)

	if err := network.SendHandshake(conn, selfID); err != nil {
		raw.Close()
		return nil, "", err
	}

	peerID, err := network.ReceiveHandshake(conn)
	if err != nil {
		raw.Close()
		return nil, "", err
	}

	return conn, peerID, nil
}

// messageLoop feeds every message received on conn to the transfer
// manager until the connection errors out (typically on close).
func messageLoop(conn *network.Conn, tm *transfer.TransferManager, peerID string) {
	for {
		msg, err := conn.Receive()
		if err != nil {
			log.Printf("node: connection to %s closed: %v", peerID, err)
			return
		}

		if err := tm.HandleNetworkMessage(msg, conn); err != nil {
			log.Printf("node: error handling %s from %s: %v", msg.Type, peerID, err)
		}
	}
}
