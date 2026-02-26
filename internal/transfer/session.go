package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/dakshcodez/sagittarius/internal/filemeta"
)

type ChunkStatus int

const (
	ChunkMissing ChunkStatus = iota
	ChunkRequested
	ChunkComplete
)

// ChunkState represents transfer-layer state for one chunk.
type ChunkState struct {
	Index         int
	Status        ChunkStatus
	RequestedFrom string
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

	// Convert missing slice to set for O(1) lookup
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

// NextChunkToRequest selects the next missing chunk and marks it requested.
func (s *DownloadSession) NextChunkToRequest() (*ChunkState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, c := range s.chunks {
		if c.Status == ChunkMissing {
			c.Status = ChunkRequested
			return c, nil
		}
	}

	return nil, errors.New("no chunks to request")
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

	// Verify chunk integrity before saving
	if err := s.verifyChunk(index, data); err != nil {
		return err
	}

	// Persist chunk to storage
	if err := s.storage.SaveChunk(s.fileID, index, data); err != nil {
		return err
	}

	// Update transfer state
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

	resp := map[string]any{
		"type":        "CHUNK_RESPONSE",
		"file_id":     s.fileID,
		"chunk_index": index,
		"data":        data,
	}

	return sender.Send(resp)
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