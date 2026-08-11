package main

import (
	"fmt"
	"io"
	"os"

	"github.com/dakshcodez/sagittarius/internal/filemeta"
	"github.com/dakshcodez/sagittarius/internal/storage"
	"github.com/dakshcodez/sagittarius/internal/transfer"
)

// seedFile hashes/chunks filePath, copies its chunk data into local
// storage, registers it with the transfer manager as a fully-complete
// download session (i.e. a seeder), and announces it to the tracker.
func seedFile(
	filePath string,
	st *storage.LocalStorage,
	tm *transfer.TransferManager,
	trackerAddr, selfID, advertiseAddr string,
) (*filemeta.FileMeta, error) {

	meta, err := filemeta.CreateFileMeta(filePath)
	if err != nil {
		return nil, fmt.Errorf("hash %s: %w", filePath, err)
	}

	if err := st.InitFileStorage(meta); err != nil {
		return nil, fmt.Errorf("init storage for %s: %w", meta.FileID, err)
	}

	if err := copyChunksIntoStorage(filePath, meta, st); err != nil {
		return nil, fmt.Errorf("copy chunks for %s: %w", meta.FileID, err)
	}

	session := transfer.NewDownloadSession(meta, st)
	tm.AddSession(session)
	tm.RegisterMeta(meta)

	if err := announceToTracker(trackerAddr, selfID, meta.FileID, advertiseAddr); err != nil {
		return nil, fmt.Errorf("announce %s to tracker: %w", meta.FileID, err)
	}

	return meta, nil
}

// copyChunksIntoStorage re-reads filePath and persists each chunk into
// storage. filemeta.CreateFileMeta only computes hashes; it does not
// retain chunk bytes, so seeding requires this second pass.
func copyChunksIntoStorage(filePath string, meta *filemeta.FileMeta, st *storage.LocalStorage) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	buf := make([]byte, meta.ChunkSize)

	for _, chunk := range meta.Chunks {
		if _, err := io.ReadFull(file, buf[:chunk.Size]); err != nil {
			return fmt.Errorf("read chunk %d: %w", chunk.Index, err)
		}

		if err := st.SaveChunk(meta.FileID, chunk.Index, buf[:chunk.Size]); err != nil {
			return err
		}
	}

	return nil
}
