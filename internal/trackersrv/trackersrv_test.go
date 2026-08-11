package trackersrv_test

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/dakshcodez/sagittarius/internal/network"
	"github.com/dakshcodez/sagittarius/internal/trackersrv"
)

func TestRegisterAnnounceLookup(t *testing.T) {
	serverConnRaw, clientConnRaw := net.Pipe()
	defer serverConnRaw.Close()
	defer clientConnRaw.Close()

	serverConn := network.NewConn(serverConnRaw)
	clientConn := network.NewConn(clientConnRaw)

	server := trackersrv.NewServer("tracker", trackersrv.NewRegistry())

	go func() {
		for {
			msg, err := serverConn.Receive()
			if err != nil {
				return
			}
			_ = server.HandleMessage(msg, serverConn)
		}
	}()

	send := func(msgType string, payload any) {
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		if err := clientConn.Send(network.Message{
			Type:     msgType,
			SenderID: "peer-A",
			Payload:  data,
		}); err != nil {
			t.Fatal(err)
		}
	}

	// REGISTER
	send(trackersrv.MsgRegister, trackersrv.RegisterPayload{
		PeerID: "peer-A",
		Addr:   "127.0.0.1:9000",
	})

	resp, err := clientConn.Receive()
	if err != nil {
		t.Fatalf("receive REGISTER_ACK failed: %v", err)
	}
	if resp.Type != trackersrv.MsgRegisterAck {
		t.Fatalf("expected %s, got %s", trackersrv.MsgRegisterAck, resp.Type)
	}

	// ANNOUNCE
	send(trackersrv.MsgAnnounce, trackersrv.AnnouncePayload{
		PeerID: "peer-A",
		FileID: "file-1",
		Addr:   "127.0.0.1:9000",
	})

	resp, err = clientConn.Receive()
	if err != nil {
		t.Fatalf("receive ANNOUNCE_ACK failed: %v", err)
	}
	if resp.Type != trackersrv.MsgAnnounceAck {
		t.Fatalf("expected %s, got %s", trackersrv.MsgAnnounceAck, resp.Type)
	}

	// LOOKUP
	send(trackersrv.MsgLookup, trackersrv.LookupPayload{FileID: "file-1"})

	resp, err = clientConn.Receive()
	if err != nil {
		t.Fatalf("receive PEER_LIST failed: %v", err)
	}
	if resp.Type != trackersrv.MsgPeerList {
		t.Fatalf("expected %s, got %s", trackersrv.MsgPeerList, resp.Type)
	}

	var peerList trackersrv.PeerListPayload
	if err := json.Unmarshal(resp.Payload, &peerList); err != nil {
		t.Fatal(err)
	}

	if len(peerList.Peers) != 1 || peerList.Peers[0].PeerID != "peer-A" {
		t.Fatalf("unexpected peer list: %+v", peerList.Peers)
	}
	if peerList.Peers[0].Addr != "127.0.0.1:9000" {
		t.Fatalf("unexpected peer addr: %+v", peerList.Peers[0])
	}
}

func TestSweepEvictsStalePeers(t *testing.T) {
	registry := trackersrv.NewRegistry()
	registry.Announce("peer-A", "file-1", "127.0.0.1:9000")

	// Not yet stale relative to a generous TTL.
	if peers := registry.Lookup("file-1", time.Minute); len(peers) != 1 {
		t.Fatalf("expected 1 peer before sweep, got %d", len(peers))
	}

	// A TTL of 0 means every peer is immediately stale.
	registry.Sweep(0)

	if peers := registry.Lookup("file-1", time.Minute); len(peers) != 0 {
		t.Fatalf("expected 0 peers after sweep, got %d", len(peers))
	}
}
