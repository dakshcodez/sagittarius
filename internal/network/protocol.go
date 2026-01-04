package network

import "github.com/dakshcodez/sagittarius/internal/filemeta"

// ---------- META ----------

type MetaRequestPayload struct {
	FileID string `json:"file_id"`
}

type MetaResponsePayload struct {
	Meta *filemeta.FileMeta `json:"meta"`
}

// ---------- CHUNKS ----------

type ChunkRequestPayload struct {
	FileID     string `json:"file_id"`
	ChunkIndex int    `json:"chunk_index"`
}

type ChunkResponsePayload struct {
	FileID     string `json:"file_id"`
	ChunkIndex int    `json:"chunk_index"`
	Data       []byte `json:"data"` // base64-encoded by JSON automatically
}
