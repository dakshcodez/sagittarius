package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dakshcodez/sagittarius/internal/filemeta"
)

// ReconstructFile merges all chunks into the final file.
func (s *LocalStorage) ReconstructFile(
	meta *filemeta.FileMeta,
	outputDir string,
) (string, error) {

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}

	outputPath := filepath.Join(outputDir, meta.FileName)

	outFile, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	// Append chunks in order
	for i := 0; i < meta.NumChunks; i++ {

		chunkPath := s.chunkPath(meta.FileID, i)

		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			return "", fmt.Errorf(
				"failed to open chunk %d: %w",
				i,
				err,
			)
		}

		_, err = io.Copy(outFile, chunkFile)

		chunkFile.Close()

		if err != nil {
			return "", fmt.Errorf(
				"failed to write chunk %d: %w",
				i,
				err,
			)
		}
	}

	return outputPath, nil
}