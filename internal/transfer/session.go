package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/dakshcodez/sagittarius/internal/filemeta"
	"github.com/dakshcodez/sagittarius/internal/network"
)

type ChunkStatus int

const (
	ChunkMissing ChunkStatus = iota
	ChunkRequested
	ChunkComplete
)

const (
	// maxOutstandingRequests bounds how many chunks can be
	// simultaneously in flight (requested but not yet completed),
	// pipelining requests instead of waiting for each response before
	// sending the next.
	maxOutstandingRequests = 8

	// chunkRequestTimeout is how long a chunk can sit in ChunkRequested
	// before the retry watchdog gives up waiting for that response and
	// makes it requestable again.
	chunkRequestTimeout = 5 * time.Second

	// maxChunkRetries caps how many times a single chunk is re-requested
	// before we stop bumping RetryCount further (it's still retried
	// indefinitely - a prototype has no other peer to fail over to
	// mid-session - but this caps the counter and lets callers notice a
	// chunk that just won't come through).
	maxChunkRetries = 20

	requestLoopIdleInterval = 20 * time.Millisecond
	retryWatchdogInterval   = 1 * time.Second
)

// ChunkState represents transfer-layer state for one chunk.
type ChunkState struct {
	Index         int
	Status        ChunkStatus
	RequestedFrom string
	RequestedAt   time.Time
	RetryCount    int
}

// DownloadSession represents one file download lifecycle.
type DownloadSession struct {
	fileID  string
	meta    *filemeta.FileMeta
	storage Storage

	mu     sync.Mutex
	chunks map[int]*ChunkState
	peers  map[string]bool
}

// NewDownloadSession initializes transfer state from storage truth.
func NewDownloadSession(
	meta *filemeta.FileMeta,
	storage Storage,
) *DownloadSession {

	s := &DownloadSession{
		fileID:  meta.FileID,
		meta:    meta,
		storage: storage,
		chunks:  make(map[int]*ChunkState),
		peers:   make(map[string]bool),
	}

	missing := storage.GetMissingChunks(meta)

	missingSet := make(map[int]bool)
	for _, m := range missing {
		missingSet[m] = true
	}

	for i := 0; i < meta.NumChunks; i++ {
		status := ChunkComplete
		if missingSet[i] {
			status = ChunkMissing
		}

		s.chunks[i] = &ChunkState{
			Index:  i,
			Status: status,
		}
	}

	return s
}

// NextChunkToRequest selects the next missing chunk and marks it
// requested, ignoring the in-flight window (used directly by tests and
// any caller that wants unpipelined, one-at-a-time behavior).
func (s *DownloadSession) NextChunkToRequest() (*ChunkState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c := s.claimNextMissingLocked()
	if c == nil {
		return nil, errors.New("no chunks to request")
	}

	return c, nil
}

// nextRequestable returns the next missing chunk to request, respecting
// maxOutstandingRequests, or ok=false if none is available right now
// (window full, or nothing currently missing).
func (s *DownloadSession) nextRequestable() (*ChunkState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	outstanding := 0
	for _, c := range s.chunks {
		if c.Status == ChunkRequested {
			outstanding++
		}
	}
	if outstanding >= maxOutstandingRequests {
		return nil, false
	}

	c := s.claimNextMissingLocked()
	return c, c != nil
}

// claimNextMissingLocked must be called with s.mu held.
func (s *DownloadSession) claimNextMissingLocked() *ChunkState {
	for _, c := range s.chunks {
		if c.Status == ChunkMissing {
			c.Status = ChunkRequested
			c.RequestedAt = time.Now()
			return c
		}
	}
	return nil
}

// requeueStaleRequests reverts any chunk that's been sitting in
// ChunkRequested longer than chunkRequestTimeout back to ChunkMissing
// (bumping its RetryCount), so a dropped request doesn't stall the
// download forever.
func (s *DownloadSession) requeueStaleRequests() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	for _, c := range s.chunks {
		if c.Status != ChunkRequested {
			continue
		}
		if now.Sub(c.RequestedAt) < chunkRequestTimeout {
			continue
		}

		if c.RetryCount < maxChunkRetries {
			c.RetryCount++
		}
		log.Printf("transfer: chunk %d timed out waiting for a response (retry %d)", c.Index, c.RetryCount)

		c.Status = ChunkMissing
		c.RequestedFrom = ""
	}
}

// IsComplete reports whether every chunk has been downloaded.
func (s *DownloadSession) IsComplete() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, c := range s.chunks {
		if c.Status != ChunkComplete {
			return false
		}
	}

	return true
}

// MarkChunkComplete marks a chunk as complete.
func (s *DownloadSession) MarkChunkComplete(index int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if c, ok := s.chunks[index]; ok {
		c.Status = ChunkComplete
		c.RequestedFrom = ""
	}
}

// HandleChunkResponse verifies and persists a received chunk.
func (s *DownloadSession) HandleChunkResponse(
	index int,
	data []byte,
) error {

	if err := s.verifyChunk(index, data); err != nil {
		log.Println("verify failed:", err)
		return err
	}

	if err := s.storage.SaveChunk(s.fileID, index, data); err != nil {
		return err
	}

	s.MarkChunkComplete(index)

	return nil
}

// HandleChunkRequest serves a chunk if available.
func (s *DownloadSession) HandleChunkRequest(
	index int,
	sender NetworkSender,
) error {

	if !s.storage.HasChunk(s.fileID, index) {
		return nil
	}

	data, err := s.storage.LoadChunk(s.fileID, index)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(struct {
		FileID     string `json:"file_id"`
		ChunkIndex int    `json:"chunk_index"`
		Data       []byte `json:"data"`
	}{
		FileID:     s.fileID,
		ChunkIndex: index,
		Data:       data,
	})
	if err != nil {
		return err
	}

	msg := network.Message{
		Type:    "CHUNK_RESPONSE",
		Payload: payload,
	}

	return sender.Send(msg)
}

// verifyChunk ensures data matches expected hash.
func (s *DownloadSession) verifyChunk(index int, data []byte) error {
	if index < 0 || index >= len(s.meta.Chunks) {
		return errors.New("invalid chunk index")
	}

	expectedHash := s.meta.Chunks[index].Hash

	sum := sha256.Sum256(data)
	actualHash := hex.EncodeToString(sum[:])

	if actualHash != expectedHash {
		return errors.New("chunk hash mismatch")
	}

	return nil
}

// StartDownload begins requesting chunks in the background: up to
// maxOutstandingRequests chunks are kept in flight at once (instead of
// waiting for each response before sending the next), and a watchdog
// re-requests any chunk whose response doesn't arrive within
// chunkRequestTimeout.
func (s *DownloadSession) StartDownload(sender NetworkSender) {
	go s.requestLoop(sender)
	go s.retryWatchdog()
}

func (s *DownloadSession) requestLoop(sender NetworkSender) {
	for !s.IsComplete() {
		chunk, ok := s.nextRequestable()
		if !ok {
			time.Sleep(requestLoopIdleInterval)
			continue
		}

		if err := s.sendChunkRequest(sender, chunk.Index); err != nil {
			return
		}
	}
}

func (s *DownloadSession) retryWatchdog() {
	ticker := time.NewTicker(retryWatchdogInterval)
	defer ticker.Stop()

	for range ticker.C {
		if s.IsComplete() {
			return
		}
		s.requeueStaleRequests()
	}
}

func (s *DownloadSession) sendChunkRequest(sender NetworkSender, index int) error {
	payload, err := json.Marshal(struct {
		FileID     string `json:"file_id"`
		ChunkIndex int    `json:"chunk_index"`
	}{
		FileID:     s.fileID,
		ChunkIndex: index,
	})
	if err != nil {
		return err
	}

	msg := network.Message{
		Type:     "CHUNK_REQUEST",
		SenderID: "downloader",
		Payload:  payload,
	}

	return sender.Send(msg)
}
