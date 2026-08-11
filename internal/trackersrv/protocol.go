package trackersrv

// Message types exchanged between a node and the tracker.
const (
	MsgRegister    = "REGISTER"
	MsgRegisterAck = "REGISTER_ACK"
	MsgAnnounce    = "ANNOUNCE"
	MsgAnnounceAck = "ANNOUNCE_ACK"
	MsgLookup      = "LOOKUP"
	MsgPeerList    = "PEER_LIST"
	MsgConnect     = "CONNECT"
	MsgPunch       = "PUNCH"
)

// PeerInfo describes one peer known to have a file, as returned by LOOKUP.
type PeerInfo struct {
	PeerID string `json:"peer_id"`
	Addr   string `json:"addr"`
}

type RegisterPayload struct {
	PeerID string `json:"peer_id"`
	Addr   string `json:"addr"`

	// LocalCandidates are the peer's own non-loopback interface
	// addresses (LAN "host candidates" in ICE terms), reported by
	// nodes registering over the QUIC/NAT path. Empty for plain-TCP
	// registrations.
	LocalCandidates []string `json:"local_candidates,omitempty"`
}

type RegisterAckPayload struct {
	OK bool `json:"ok"`

	// ReflexiveAddr is this peer's public address as observed by the
	// tracker (the QUIC connection's remote address) - i.e. a STUN
	// binding response piggybacked on registration. Empty for
	// plain-TCP registrations.
	ReflexiveAddr string `json:"reflexive_addr,omitempty"`
}

// ConnectPayload requests that the tracker broker a connection to another
// registered peer: the tracker replies with that peer's candidates (as a
// PUNCH message) and, if the target is reachable, pushes the requester's
// own candidates to the target the same way.
type ConnectPayload struct {
	TargetPeerID string `json:"target_peer_id"`
}

// PunchPayload carries one peer's dial candidates - either as the direct
// reply to a CONNECT, or pushed unprompted to the target of someone
// else's CONNECT. A nil/empty Candidates means the tracker has no record
// of PeerID (e.g. it isn't currently registered).
type PunchPayload struct {
	PeerID     string   `json:"peer_id"`
	Candidates []string `json:"candidates"`
}

type AnnouncePayload struct {
	PeerID string `json:"peer_id"`
	FileID string `json:"file_id"`
	Addr   string `json:"addr"`
}

type AnnounceAckPayload struct {
	OK bool `json:"ok"`
}

type LookupPayload struct {
	FileID string `json:"file_id"`
}

type PeerListPayload struct {
	Peers []PeerInfo `json:"peers"`
}
