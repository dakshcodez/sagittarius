package trackersrv

import (
	"encoding/json"

	"github.com/dakshcodez/sagittarius/internal/network"
)

// Sender abstracts sending a reply back to whichever peer sent the request.
type Sender interface {
	Send(msg any) error
}

// Server dispatches incoming tracker protocol messages against a Registry.
type Server struct {
	SelfID   string
	Registry *Registry
}

func NewServer(selfID string, registry *Registry) *Server {
	return &Server{SelfID: selfID, Registry: registry}
}

// HandleMessage processes one tracker-protocol message and, where the
// protocol calls for it, sends a reply via sender.
func (s *Server) HandleMessage(msg *network.Message, sender Sender) error {
	switch msg.Type {

	case MsgRegister:
		var payload RegisterPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return err
		}

		s.Registry.Register(payload.PeerID, payload.Addr)

		return s.reply(sender, MsgRegisterAck, RegisterAckPayload{OK: true})

	case MsgAnnounce:
		var payload AnnouncePayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return err
		}

		s.Registry.Announce(payload.PeerID, payload.FileID, payload.Addr)

		return s.reply(sender, MsgAnnounceAck, AnnounceAckPayload{OK: true})

	case MsgLookup:
		var payload LookupPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return err
		}

		peers := s.Registry.Lookup(payload.FileID, DefaultTTL)

		return s.reply(sender, MsgPeerList, PeerListPayload{Peers: peers})

	default:
		// Ignore anything the tracker doesn't understand.
		return nil
	}
}

func (s *Server) reply(sender Sender, msgType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return sender.Send(network.Message{
		Type:     msgType,
		SenderID: s.SelfID,
		Payload:  data,
	})
}
