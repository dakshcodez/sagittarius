package transfer

import "sync"

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