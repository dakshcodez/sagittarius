package storage

import (
	"encoding/json"
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

func (s *LocalStorage) metaPath(fileID string) string {
	return filepath.Join(s.baseDir, fileID, "meta.json")
}

// InitFileStorage creates the on-disk layout for a file (chunks dir +
// meta.json) so chunks can subsequently be saved/loaded against it.
func (s *LocalStorage) InitFileStorage(meta *filemeta.FileMeta) error {
	if err := s.ensureChunkDir(meta.FileID); err != nil {
		return err
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.metaPath(meta.FileID), data, 0644)
}

// LoadFileMeta reads back a previously initialized file's metadata.
func (s *LocalStorage) LoadFileMeta(fileID string) (*filemeta.FileMeta, error) {
	data, err := os.ReadFile(s.metaPath(fileID))
	if err != nil {
		return nil, err
	}

	var meta filemeta.FileMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
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
