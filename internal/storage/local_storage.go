package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dakshcodez/sagittarius/internal/filemeta"
)

type LocalStorage struct {
	baseDir string
}

func NewLocalStorage(baseDir string) *LocalStorage {
	return &LocalStorage{
		baseDir: baseDir,
	}
}

func (s *LocalStorage) chunkPath(fileID string, index int) string {
	return filepath.Join(
		s.baseDir,
		fileID,
		"chunks",
		fmt.Sprintf("%d.chunk", index),
	)
}

func (s *LocalStorage) ensureChunkDir(fileID string) error {
	dir := filepath.Join(s.baseDir, fileID, "chunks")
	return os.MkdirAll(dir, 0755)
}

func (s *LocalStorage) HasChunk(fileID string, index int) bool {
	path := s.chunkPath(fileID, index)

	_, err := os.Stat(path)
	return err == nil
}

func (s *LocalStorage) LoadChunk(fileID string, index int) ([]byte, error) {
	path := s.chunkPath(fileID, index)
	return os.ReadFile(path)
}

func (s *LocalStorage) SaveChunk(
	fileID string,
	index int,
	data []byte,
) error {

	if err := s.ensureChunkDir(fileID); err != nil {
		return err
	}

	final := s.chunkPath(fileID, index)
	tmp := final + ".tmp"

	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmp, final)
}

func (s *LocalStorage) GetMissingChunks(
	meta *filemeta.FileMeta,
) []int {

	var missing []int

	for i := 0; i < meta.NumChunks; i++ {
		if !s.HasChunk(meta.FileID, i) {
			missing = append(missing, i)
		}
	}

	return missing
}