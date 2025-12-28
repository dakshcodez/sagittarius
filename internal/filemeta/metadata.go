package filemeta

type ChunkMeta struct {
	Index int    `json:"index"`
	Hash  string `json:"hash"`
	Size  int    `json:"size"`
}

type FileMeta struct {
	FileID    string  	  `json:"file_id"`
	FileName  string 	  `json:"file_name"`
	FileSize  int	 	  `json:"file_size"`
	ChunkSize int	 	  `json:"chunk_size"`
	NumChunks int	 	  `json:"num_chunks"`
	Chunks	  []ChunkMeta `json:"chunks"`
}