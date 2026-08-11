package trackersrv

// Message types exchanged between a node and the tracker.
const (
	MsgRegister    = "REGISTER"
	MsgRegisterAck = "REGISTER_ACK"
	MsgAnnounce    = "ANNOUNCE"
	MsgAnnounceAck = "ANNOUNCE_ACK"
	MsgLookup      = "LOOKUP"
	MsgPeerList    = "PEER_LIST"
)

// PeerInfo describes one peer known to have a file, as returned by LOOKUP.
type PeerInfo struct {
	PeerID string `json:"peer_id"`
	Addr   string `json:"addr"`
}

type RegisterPayload struct {
	PeerID string `json:"peer_id"`
	Addr   string `json:"addr"`
}

type RegisterAckPayload struct {
	OK bool `json:"ok"`
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
