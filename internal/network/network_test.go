package network

import (
	"net"
	"testing"
)

func TestSendReceiveMessage(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	conn1 := NewConn(c1)
	conn2 := NewConn(c2)

	msg := Message{
		Type:     "PING",
		SenderID: "node-1",
		Payload:  []byte(`{"hello":"world"}`),
	}

	// send in goroutine because Receive blocks
	go func() {
		if err := conn1.Send(msg); err != nil {
			t.Errorf("Send failed: %v", err)
		}
	}()

	received, err := conn2.Receive()
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	if received.Type != msg.Type {
		t.Fatalf("expected type %s, got %s", msg.Type, received.Type)
	}

	if received.SenderID != msg.SenderID {
		t.Fatalf("expected sender %s, got %s", msg.SenderID, received.SenderID)
	}
}

func TestHandshakeSuccess(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	conn1 := NewConn(c1)
	conn2 := NewConn(c2)

	go func() {
		if err := SendHandshake(conn1, "peer-A"); err != nil {
			t.Errorf("SendHandshake failed: %v", err)
		}
	}()

	peerID, err := ReceiveHandshake(conn2)
	if err != nil {
		t.Fatalf("ReceiveHandshake failed: %v", err)
	}

	if peerID != "peer-A" {
		t.Fatalf("expected peer-A, got %s", peerID)
	}
}

func TestHandshakeRejectsWrongMessage(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	conn1 := NewConn(c1)
	conn2 := NewConn(c2)

	go func() {
		conn1.Send(Message{
			Type:     "PING",
			SenderID: "evil-peer",
			Payload:  []byte(`{}`),
		})
	}()

	_, err := ReceiveHandshake(conn2)
	if err == nil {
		t.Fatal("expected handshake error, got nil")
	}
}
