package trackersrv

import (
	"sync"
	"time"
)

const (
	DefaultTTL           = 90 * time.Second
	DefaultSweepInterval = 30 * time.Second
)

// Pusher lets the tracker proactively deliver a message to a registered
// peer outside of any request/response exchange - used to signal PUNCH
// to the target of someone else's CONNECT. Implemented by cmd/tracker's
// QUIC-serving code (opens a new stream on the peer's control
// connection); nil for peers registered over the plain-TCP path, which
// have no persistent connection to push on.
type Pusher interface {
	Push(msgType string, payload any) error
}

type peerEntry struct {
	peerID string

	addr            string // TCP dialable addr (plain-TCP registration path)
	reflexiveAddr   string // tracker-observed public addr (QUIC/NAT path)
	localCandidates []string
	pusher          Pusher

	lastSeen time.Time
}

// Registry is the tracker's in-memory peer/file directory.
type Registry struct {
	mu    sync.Mutex
	peers map[string]*peerEntry      // peer_id -> entry
	files map[string]map[string]bool // file_id -> set of peer_ids
}

func NewRegistry() *Registry {
	return &Registry{
		peers: make(map[string]*peerEntry),
		files: make(map[string]map[string]bool),
	}
}

func (r *Registry) entry(peerID string) *peerEntry {
	e, ok := r.peers[peerID]
	if !ok {
		e = &peerEntry{peerID: peerID}
		r.peers[peerID] = e
	}
	return e
}

// Register records/refreshes a peer's dialable TCP address (the plain-TCP
// path from Phase 1).
func (r *Registry) Register(peerID, addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	e := r.entry(peerID)
	e.addr = addr
	e.lastSeen = time.Now()
}

// RegisterNAT records/refreshes a peer's NAT-traversal candidates and the
// live connection the tracker can push PUNCH signals on (the QUIC path
// from Phase 2). Returns the tracker-observed reflexive address so the
// caller can report it back to the peer as a STUN-style binding response.
func (r *Registry) RegisterNAT(peerID, reflexiveAddr string, localCandidates []string, pusher Pusher) {
	r.mu.Lock()
	defer r.mu.Unlock()

	e := r.entry(peerID)
	e.reflexiveAddr = reflexiveAddr
	e.localCandidates = localCandidates
	e.pusher = pusher
	e.lastSeen = time.Now()
}

// Announce records that a peer has a given file, implicitly registering
// its (plain-TCP) address.
func (r *Registry) Announce(peerID, fileID, addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	e := r.entry(peerID)
	e.addr = addr
	e.lastSeen = time.Now()

	if r.files[fileID] == nil {
		r.files[fileID] = make(map[string]bool)
	}
	r.files[fileID][peerID] = true
}

// Lookup returns known, non-stale peers advertising a given file.
func (r *Registry) Lookup(fileID string, ttl time.Duration) []PeerInfo {
	r.mu.Lock()
	defer r.mu.Unlock()

	var result []PeerInfo
	now := time.Now()

	for peerID := range r.files[fileID] {
		entry, ok := r.peers[peerID]
		if !ok {
			continue
		}
		if now.Sub(entry.lastSeen) > ttl {
			continue
		}
		result = append(result, PeerInfo{PeerID: entry.peerID, Addr: entry.addr})
	}

	return result
}

// Candidates returns the full set of dial candidates known for peerID:
// its reported LAN addresses plus its tracker-observed reflexive address,
// for use in a NAT punch. Returns ok=false if the peer isn't registered.
func (r *Registry) Candidates(peerID string) (candidates []string, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	e, exists := r.peers[peerID]
	if !exists {
		return nil, false
	}

	candidates = append(candidates, e.localCandidates...)
	if e.reflexiveAddr != "" {
		candidates = append(candidates, e.reflexiveAddr)
	}

	return candidates, true
}

// PushTo delivers a message to peerID via its registered Pusher, if it
// has one (i.e. it's registered over the QUIC/NAT path and currently
// connected). Returns false if there is nothing to push to.
func (r *Registry) PushTo(peerID, msgType string, payload any) (delivered bool, err error) {
	r.mu.Lock()
	e, ok := r.peers[peerID]
	r.mu.Unlock()

	if !ok || e.pusher == nil {
		return false, nil
	}

	return true, e.pusher.Push(msgType, payload)
}

// Sweep evicts peers (and their file associations) not seen within ttl.
// Intended to be called periodically by a background goroutine.
func (r *Registry) Sweep(ttl time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	for peerID, entry := range r.peers {
		if now.Sub(entry.lastSeen) <= ttl {
			continue
		}

		delete(r.peers, peerID)

		for fileID, peerSet := range r.files {
			delete(peerSet, peerID)
			if len(peerSet) == 0 {
				delete(r.files, fileID)
			}
		}
	}
}

// StartSweeper runs Sweep on interval until ctx is done. It is the caller's
// responsibility to cancel ctx to stop the goroutine.
func (r *Registry) StartSweeper(done <-chan struct{}, ttl, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				r.Sweep(ttl)
			}
		}
	}()
}
