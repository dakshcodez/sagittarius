package transfer

import (
	"encoding/json"
	"sync"

	"github.com/dakshcodez/sagittarius/internal/network"
)

// TransferManager coordinates all active download sessions.
type TransferManager struct {
	selfID string

	mu       sync.Mutex
	sessions map[string]*DownloadSession // fileID -> session
}

// NewTransferManager initializes a new transfer manager.
func NewTransferManager(selfID string) *TransferManager {
	return &TransferManager{
		selfID:   selfID,
		sessions: make(map[string]*DownloadSession),
	}
}

// GetSession retrieves a session by fileID.
func (tm *TransferManager) GetSession(fileID string) (*DownloadSession, bool) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	s, ok := tm.sessions[fileID]
	return s, ok
}

// AddSession registers a new download session.
func (tm *TransferManager) AddSession(s *DownloadSession) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.sessions[s.fileID] = s
}

// HandleNetworkMessage routes incoming network messages to the appropriate session.
func (tm *TransferManager) HandleNetworkMessage(
	msg *network.Message,
	sender NetworkSender,
) error {

	switch msg.Type {

	case "CHUNK_REQUEST":

		var payload struct {
			FileID     string `json:"file_id"`
			ChunkIndex int    `json:"chunk_index"`
		}

		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return err
		}

		session, ok := tm.GetSession(payload.FileID)
		if !ok {
			return nil
		}

		return session.HandleChunkRequest(payload.ChunkIndex, sender)

	case "CHUNK_RESPONSE":

		var payload struct {
			FileID     string `json:"file_id"`
			ChunkIndex int    `json:"chunk_index"`
			Data       []byte `json:"data"`
		}

		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return err
		}

		session, ok := tm.GetSession(payload.FileID)
		if !ok {
			return nil
		}

		return session.HandleChunkResponse(payload.ChunkIndex, payload.Data)

	default:
		// Ignore messages not handled by transfer layer
		return nil
	}
}