package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/dakshcodez/sagittarius/internal/nat"
	"github.com/dakshcodez/sagittarius/internal/network"
	"github.com/dakshcodez/sagittarius/internal/quicconn"
	"github.com/dakshcodez/sagittarius/internal/trackersrv"
	"github.com/quic-go/quic-go"
)

const (
	natKeepaliveInterval = 25 * time.Second
	natRequestTimeout    = 10 * time.Second
)

// natClient is a node's persistent QUIC control connection to the
// tracker: one dial, then a fresh stream per request (REGISTER, LOOKUP,
// CONNECT, ...), plus a background loop accepting pushed streams
// (unprompted PUNCH signals) the tracker opens back at us.
type natClient struct {
	socket      *nat.Socket
	trackerConn *quic.Conn
	selfID      string
}

func dialTrackerNAT(socket *nat.Socket, trackerAddr, selfID string) (*natClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), natRequestTimeout)
	defer cancel()

	trackerConn, err := socket.Dial(ctx, trackerAddr)
	if err != nil {
		return nil, fmt.Errorf("dial tracker over quic: %w", err)
	}

	return &natClient{socket: socket, trackerConn: trackerConn, selfID: selfID}, nil
}

// request opens a fresh stream, sends one message, and returns the reply.
func (c *natClient) request(ctx context.Context, msgType string, payload any) (*network.Message, error) {
	streamConn, err := quicconn.OpenStream(c.trackerConn)
	if err != nil {
		return nil, fmt.Errorf("open stream: %w", err)
	}
	defer streamConn.Close()

	conn := network.NewConn(streamConn)

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	if err := conn.Send(network.Message{
		Type:     msgType,
		SenderID: c.selfID,
		Payload:  data,
	}); err != nil {
		return nil, fmt.Errorf("send %s: %w", msgType, err)
	}

	resp, err := conn.Receive()
	if err != nil {
		return nil, fmt.Errorf("receive reply to %s: %w", msgType, err)
	}

	return resp, nil
}

func (c *natClient) register(localCandidates []string) (reflexiveAddr string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), natRequestTimeout)
	defer cancel()

	resp, err := c.request(ctx, trackersrv.MsgRegister, trackersrv.RegisterPayload{
		PeerID:          c.selfID,
		LocalCandidates: localCandidates,
	})
	if err != nil {
		return "", err
	}

	var ack trackersrv.RegisterAckPayload
	if err := json.Unmarshal(resp.Payload, &ack); err != nil {
		return "", err
	}

	return ack.ReflexiveAddr, nil
}

func (c *natClient) announce(fileID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), natRequestTimeout)
	defer cancel()

	_, err := c.request(ctx, trackersrv.MsgAnnounce, trackersrv.AnnouncePayload{
		PeerID: c.selfID,
		FileID: fileID,
	})
	return err
}

func (c *natClient) lookup(fileID string) ([]trackersrv.PeerInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), natRequestTimeout)
	defer cancel()

	resp, err := c.request(ctx, trackersrv.MsgLookup, trackersrv.LookupPayload{FileID: fileID})
	if err != nil {
		return nil, err
	}

	var list trackersrv.PeerListPayload
	if err := json.Unmarshal(resp.Payload, &list); err != nil {
		return nil, err
	}

	return list.Peers, nil
}

// connect asks the tracker to broker a connection to targetPeerID and
// returns that peer's dial candidates.
func (c *natClient) connect(targetPeerID string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), natRequestTimeout)
	defer cancel()

	resp, err := c.request(ctx, trackersrv.MsgConnect, trackersrv.ConnectPayload{
		TargetPeerID: targetPeerID,
	})
	if err != nil {
		return nil, err
	}

	var punch trackersrv.PunchPayload
	if err := json.Unmarshal(resp.Payload, &punch); err != nil {
		return nil, err
	}

	if len(punch.Candidates) == 0 {
		return nil, fmt.Errorf("tracker has no candidates for peer %s", targetPeerID)
	}

	return punch.Candidates, nil
}

// keepalive periodically re-registers so the tracker's TTL sweep doesn't
// evict us, and so the NAT mapping the tracker observed stays open.
func (c *natClient) keepalive(localCandidates []string) {
	ticker := time.NewTicker(natKeepaliveInterval)
	defer ticker.Stop()

	for range ticker.C {
		if _, err := c.register(localCandidates); err != nil {
			log.Printf("node: nat keepalive failed: %v", err)
		}
	}
}

// runPushListener accepts streams the tracker opens on our control
// connection (unprompted PUNCH signals naming a peer who wants to
// connect to us) and hands each off to onPunch.
func (c *natClient) runPushListener(onPunch func(peerID string, candidates []string)) {
	ctx := c.trackerConn.Context()

	for {
		stream, err := c.trackerConn.AcceptStream(ctx)
		if err != nil {
			return
		}

		go func() {
			conn := network.NewConn(quicconn.Wrap(c.trackerConn, stream))

			msg, err := conn.Receive()
			if err != nil {
				return
			}
			if msg.Type != trackersrv.MsgPunch {
				return
			}

			var p trackersrv.PunchPayload
			if err := json.Unmarshal(msg.Payload, &p); err != nil {
				return
			}

			onPunch(p.PeerID, p.Candidates)
		}()
	}
}
