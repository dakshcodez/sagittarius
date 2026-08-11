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
			_ = server.HandleMessage(msg, serverConn, trackersrv.RequestContext{})
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

// fakePusher records every message pushed to it, standing in for
// cmd/tracker's real QUIC-stream-opening Pusher.
type fakePusher struct {
	pushes []pushedMsg
}

type pushedMsg struct {
	msgType string
	payload any
}

func (f *fakePusher) Push(msgType string, payload any) error {
	f.pushes = append(f.pushes, pushedMsg{msgType: msgType, payload: payload})
	return nil
}

func (f *fakePusher) OpenStream(msgType string, payload any) (net.Conn, error) {
	f.pushes = append(f.pushes, pushedMsg{msgType: msgType, payload: payload})
	a, _ := net.Pipe()
	return a, nil
}

func TestConnectSignalsPunchBothWays(t *testing.T) {
	registry := trackersrv.NewRegistry()
	server := trackersrv.NewServer("tracker", registry)

	pusherB := &fakePusher{}
	registry.RegisterNAT("peer-A", "203.0.113.1:4000", []string{"10.0.0.1:4000"}, nil)
	registry.RegisterNAT("peer-B", "203.0.113.2:5000", []string{"10.0.0.2:5000"}, pusherB)

	serverConnRaw, clientConnRaw := net.Pipe()
	defer serverConnRaw.Close()
	defer clientConnRaw.Close()

	serverConn := network.NewConn(serverConnRaw)
	clientConn := network.NewConn(clientConnRaw)

	go func() {
		msg, err := serverConn.Receive()
		if err != nil {
			return
		}
		_ = server.HandleMessage(msg, serverConn, trackersrv.RequestContext{})
	}()

	data, err := json.Marshal(trackersrv.ConnectPayload{TargetPeerID: "peer-B"})
	if err != nil {
		t.Fatal(err)
	}
	if err := clientConn.Send(network.Message{
		Type:     trackersrv.MsgConnect,
		SenderID: "peer-A",
		Payload:  data,
	}); err != nil {
		t.Fatal(err)
	}

	resp, err := clientConn.Receive()
	if err != nil {
		t.Fatalf("receive PUNCH reply failed: %v", err)
	}
	if resp.Type != trackersrv.MsgPunch {
		t.Fatalf("expected %s, got %s", trackersrv.MsgPunch, resp.Type)
	}

	var punch trackersrv.PunchPayload
	if err := json.Unmarshal(resp.Payload, &punch); err != nil {
		t.Fatal(err)
	}

	if punch.PeerID != "peer-B" {
		t.Fatalf("expected candidates for peer-B, got %s", punch.PeerID)
	}
	wantCandidates := map[string]bool{"10.0.0.2:5000": true, "203.0.113.2:5000": true}
	if len(punch.Candidates) != len(wantCandidates) {
		t.Fatalf("unexpected candidates: %v", punch.Candidates)
	}
	for _, c := range punch.Candidates {
		if !wantCandidates[c] {
			t.Fatalf("unexpected candidate %q in %v", c, punch.Candidates)
		}
	}

	// Give the goroutine handling CONNECT a moment to also push to B.
	deadline := time.Now().Add(time.Second)
	for len(pusherB.pushes) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	if len(pusherB.pushes) != 1 {
		t.Fatalf("expected 1 push to peer-B, got %d", len(pusherB.pushes))
	}
	if pusherB.pushes[0].msgType != trackersrv.MsgPunch {
		t.Fatalf("expected pushed type %s, got %s", trackersrv.MsgPunch, pusherB.pushes[0].msgType)
	}

	pushedPunch, ok := pusherB.pushes[0].payload.(trackersrv.PunchPayload)
	if !ok {
		t.Fatalf("unexpected pushed payload type: %T", pusherB.pushes[0].payload)
	}
	if pushedPunch.PeerID != "peer-A" {
		t.Fatalf("expected pushed candidates for peer-A, got %s", pushedPunch.PeerID)
	}
}

func TestConnectUnknownTargetReturnsEmptyCandidates(t *testing.T) {
	registry := trackersrv.NewRegistry()
	server := trackersrv.NewServer("tracker", registry)

	serverConnRaw, clientConnRaw := net.Pipe()
	defer serverConnRaw.Close()
	defer clientConnRaw.Close()

	serverConn := network.NewConn(serverConnRaw)
	clientConn := network.NewConn(clientConnRaw)

	go func() {
		msg, err := serverConn.Receive()
		if err != nil {
			return
		}
		_ = server.HandleMessage(msg, serverConn, trackersrv.RequestContext{})
	}()

	data, _ := json.Marshal(trackersrv.ConnectPayload{TargetPeerID: "ghost"})
	if err := clientConn.Send(network.Message{
		Type:     trackersrv.MsgConnect,
		SenderID: "peer-A",
		Payload:  data,
	}); err != nil {
		t.Fatal(err)
	}

	resp, err := clientConn.Receive()
	if err != nil {
		t.Fatalf("receive PUNCH reply failed: %v", err)
	}

	var punch trackersrv.PunchPayload
	if err := json.Unmarshal(resp.Payload, &punch); err != nil {
		t.Fatal(err)
	}
	if len(punch.Candidates) != 0 {
		t.Fatalf("expected no candidates for unknown peer, got %v", punch.Candidates)
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
