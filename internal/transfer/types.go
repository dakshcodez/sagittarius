package transfer

import "github.com/dakshcodez/sagittarius/internal/filemeta"

// Storage defines the contract required by the transfer layer.
// Transfer does NOT know how storage is implemented.
type Storage interface {
	HasChunk(fileID string, index int) bool
	LoadChunk(fileID string, index int) ([]byte, error)
	SaveChunk(fileID string, index int, data []byte) error
	GetMissingChunks(meta *filemeta.FileMeta) []int
}

// NetworkSender abstracts the network layer.
// Transfer decides WHAT to send, not HOW it is sent.
type NetworkSender interface {
	Send(msg any) error
}