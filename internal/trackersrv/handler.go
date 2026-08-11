package trackersrv

import (
	"encoding/json"

	"github.com/dakshcodez/sagittarius/internal/network"
)

// Sender abstracts sending a reply back to whichever peer sent the request.
type Sender interface {
	Send(msg any) error
}

// RequestContext carries transport-specific information the QUIC/NAT path
// needs that the plain-TCP path doesn't have: the connection's observed
// remote address (used as a STUN-style reflexive address on REGISTER) and
// a Pusher for delivering later unprompted PUNCH signals to this peer.
// Both are the zero value for plain-TCP requests.
type RequestContext struct {
	ReflexiveAddr string
	Pusher        Pusher
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
func (s *Server) HandleMessage(msg *network.Message, sender Sender, rc RequestContext) error {
	switch msg.Type {

	case MsgRegister:
		var payload RegisterPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return err
		}

		s.Registry.Register(payload.PeerID, payload.Addr)

		if rc.ReflexiveAddr != "" || rc.Pusher != nil {
			s.Registry.RegisterNAT(payload.PeerID, rc.ReflexiveAddr, payload.LocalCandidates, rc.Pusher)
		}

		return s.reply(sender, MsgRegisterAck, RegisterAckPayload{
			OK:            true,
			ReflexiveAddr: rc.ReflexiveAddr,
		})

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

	case MsgConnect:
		var payload ConnectPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return err
		}

		targetCandidates, ok := s.Registry.Candidates(payload.TargetPeerID)
		if !ok {
			return s.reply(sender, MsgPunch, PunchPayload{PeerID: payload.TargetPeerID})
		}

		if err := s.reply(sender, MsgPunch, PunchPayload{
			PeerID:     payload.TargetPeerID,
			Candidates: targetCandidates,
		}); err != nil {
			return err
		}

		requesterCandidates, _ := s.Registry.Candidates(msg.SenderID)
		_, err := s.Registry.PushTo(payload.TargetPeerID, MsgPunch, PunchPayload{
			PeerID:     msg.SenderID,
			Candidates: requesterCandidates,
		})
		return err

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
