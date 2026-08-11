package trackersrv

import (
	"sync"
	"time"
)

const (
	DefaultTTL           = 90 * time.Second
	DefaultSweepInterval = 30 * time.Second
)

type peerEntry struct {
	peerID   string
	addr     string
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

// Register records/refreshes a peer's dialable address.
func (r *Registry) Register(peerID, addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.peers[peerID] = &peerEntry{
		peerID:   peerID,
		addr:     addr,
		lastSeen: time.Now(),
	}
}

// Announce records that a peer has a given file, implicitly registering it.
func (r *Registry) Announce(peerID, fileID, addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.peers[peerID] = &peerEntry{
		peerID:   peerID,
		addr:     addr,
		lastSeen: time.Now(),
	}

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
