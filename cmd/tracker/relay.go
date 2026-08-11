package main

import (
	"encoding/json"
	"io"
	"log"
	"net"

	"github.com/dakshcodez/sagittarius/internal/network"
	"github.com/dakshcodez/sagittarius/internal/trackersrv"
)

// handleRelay is reached when a stream's first message was RELAY: rawA is
// the requester's half of the pipe (with the RELAY header already
// consumed from it). It opens the matching half on the target's control
// connection - with a RELAY header of its own, so the target's push
// listener knows to treat it as a data connection rather than a PUNCH
// signal - and splices the two raw streams together. From here on the
// tracker relays opaque bytes; it never parses this traffic.
func handleRelay(registry *trackersrv.Registry, rawA net.Conn, msg *network.Message) {
	var payload trackersrv.RelayPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		rawA.Close()
		return
	}

	rawB, err := registry.OpenRelayStream(payload.TargetPeerID, trackersrv.MsgRelay, trackersrv.RelayPayload{
		TargetPeerID: msg.SenderID,
	})
	if err != nil {
		log.Printf("tracker: relay %s -> %s failed: %v", msg.SenderID, payload.TargetPeerID, err)
		rawA.Close()
		return
	}

	log.Printf("tracker: relaying %s <-> %s", msg.SenderID, payload.TargetPeerID)
	splice(rawA, rawB)
}

// splice pipes bytes bidirectionally between a and b until either side
// closes, then closes both.
func splice(a, b net.Conn) {
	done := make(chan struct{}, 2)

	go func() {
		io.Copy(a, b)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(b, a)
		done <- struct{}{}
	}()

	<-done
	a.Close()
	b.Close()
}
