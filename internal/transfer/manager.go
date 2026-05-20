package transfer

import (
	"encoding/json"
	"sync"

	"github.com/dakshcodez/sagittarius/internal/filemeta"
	"github.com/dakshcodez/sagittarius/internal/network"
)

// TransferManager coordinates all active download sessions.
type TransferManager struct {
	selfID string

	mu       sync.Mutex
	sessions map[string]*DownloadSession // fileID -> session
	metas    map[string]*filemeta.FileMeta
}

// NewTransferManager initializes a new transfer manager.
func NewTransferManager(selfID string) *TransferManager {
	return &TransferManager{
		selfID:   selfID,
		sessions: make(map[string]*DownloadSession),
		metas:    make(map[string]*filemeta.FileMeta),
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

	case "META_REQUEST":

		var payload struct {
			FileID string `json:"file_id"`
		}

		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return err
		}

		tm.mu.Lock()
		meta, ok := tm.metas[payload.FileID]
		tm.mu.Unlock()

		if !ok {
			return nil
		}

		metaPayload, err := json.Marshal(meta)
		if err != nil {
			return err
		}

		resp := network.Message{
			Type:     "META_RESPONSE",
			SenderID: tm.selfID,
			Payload:  metaPayload,
		}

		return sender.Send(resp)	

	case "META_RESPONSE":

		var meta filemeta.FileMeta

		if err := json.Unmarshal(msg.Payload, &meta); err != nil {
			return err
		}

		tm.RegisterMeta(&meta)

		return nil	

	default:
		// Ignore messages not handled by transfer layer
		return nil
	}
}

func (tm *TransferManager) RegisterMeta(meta *filemeta.FileMeta) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.metas[meta.FileID] = meta
}

func (tm *TransferManager) RequestMeta(
	fileID string,
	sender NetworkSender,
) error {

	payload, err := json.Marshal(struct {
		FileID string `json:"file_id"`
	}{
		FileID: fileID,
	})
	if err != nil {
		return err
	}

	msg := network.Message{
		Type:     "META_REQUEST",
		SenderID: tm.selfID,
		Payload:  payload,
	}

	return sender.Send(msg)
}