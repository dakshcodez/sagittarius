package main

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/dakshcodez/sagittarius/internal/network"
	"github.com/dakshcodez/sagittarius/internal/trackersrv"
)

// trackerRequest opens a short-lived connection to the tracker, sends one
// message, and returns the single reply. Phase 1 keeps tracker
// communication request/response over plain TCP; Phase 2 replaces this
// with a persistent QUIC control connection.
func trackerRequest(trackerAddr, selfID, msgType string, payload any) (*network.Message, error) {
	raw, err := net.Dial("tcp", trackerAddr)
	if err != nil {
		return nil, fmt.Errorf("dial tracker: %w", err)
	}
	defer raw.Close()

	conn := network.NewConn(raw)

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	if err := conn.Send(network.Message{
		Type:     msgType,
		SenderID: selfID,
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

func registerWithTracker(trackerAddr, selfID, advertiseAddr string) error {
	_, err := trackerRequest(trackerAddr, selfID, trackersrv.MsgRegister, trackersrv.RegisterPayload{
		PeerID: selfID,
		Addr:   advertiseAddr,
	})
	return err
}

func announceToTracker(trackerAddr, selfID, fileID, advertiseAddr string) error {
	_, err := trackerRequest(trackerAddr, selfID, trackersrv.MsgAnnounce, trackersrv.AnnouncePayload{
		PeerID: selfID,
		FileID: fileID,
		Addr:   advertiseAddr,
	})
	return err
}

func lookupPeers(trackerAddr, selfID, fileID string) ([]trackersrv.PeerInfo, error) {
	resp, err := trackerRequest(trackerAddr, selfID, trackersrv.MsgLookup, trackersrv.LookupPayload{
		FileID: fileID,
	})
	if err != nil {
		return nil, err
	}

	if resp.Type != trackersrv.MsgPeerList {
		return nil, fmt.Errorf("unexpected tracker reply type %q", resp.Type)
	}

	var list trackersrv.PeerListPayload
	if err := json.Unmarshal(resp.Payload, &list); err != nil {
		return nil, err
	}

	return list.Peers, nil
}
