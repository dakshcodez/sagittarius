package filemeta

import (
	"io"
	"os"
)

const DefaultChunkSize = 1 * 1024 * 1024	// 1 MB

func CreateFileMeta(filePath string) (*FileMeta, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	var chunks []ChunkMeta
	buffer := make([]byte, DefaultChunkSize)

	index := 0

	for {
		n, err := file.Read(buffer)
		
		if n > 0 {
			data := buffer[:n]
			hash := HashBytes(data)

			chunks = append(chunks, ChunkMeta{
				Index: index,
				Hash: hash,
				Size: n,
			})

			index++
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}
	}

	meta := &FileMeta{
		FileName: info.Name(),
		FileSize: int(info.Size()),
		ChunkSize: DefaultChunkSize,
		NumChunks: len(chunks),
		Chunks: chunks,
	}

	meta.FileID = computeFileID(meta)

	return meta, nil
}

func computeFileID(meta *FileMeta) string {
	builder := ""
	for _, c := range meta.Chunks {
		builder += c.Hash
	}
	return HashBytes([]byte(builder))
}