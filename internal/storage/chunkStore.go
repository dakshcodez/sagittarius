package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dakshcodez/sagittarius/internal/filemeta"
)

func InitFileStorage(meta *filemeta.FileMeta) error {
	if err := os.MkdirAll(chunksDir(meta.FileID), 0755); err != nil {
		return err
	}

	metaBytes, err := jsonMarshal(meta)
	if err != nil {
		return err
	}

	if err := os.WriteFile(metaPath(meta.FileID), metaBytes, 0644); err != nil {
		return err
	}

	state, _ := loadState(statePath(meta.FileID))
	return saveState(statePath(meta.FileID), state)
}

func HasChunk(fileID string, index int) bool {
	path := filepath.Join(chunksDir(fileID), chunkName(index))

	_, err := os.Stat(path)
	return err == nil
}

func SaveChunk(fileID string, index int, data []byte) error {
	tmp := filepath.Join(chunksDir(fileID), chunkName(index)+".tmp")
	final := filepath.Join(chunksDir(fileID), chunkName(index))

	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}

	if err := os.Rename(tmp, final); err != nil {
		return err
	}

	state, err := loadState(statePath(fileID))
	if err != nil {
		return err
	}

	state.Downloaded[index] = true
	return saveState(statePath(fileID), state)
}

func LoadChunk(fileID string, index int) ([]byte, error) {
	path := filepath.Join(chunksDir(fileID), chunkName(index))
	if _, err := os.Stat(path); err != nil {
		return nil, errors.New("chunks not found")
	}
	return os.ReadFile(path)
}

func GetMissingChunks(meta *filemeta.FileMeta) []int {
	state, _ := loadState(statePath(meta.FileID))
	var missing []int

	for i:= 0; i < meta.NumChunks; i++ {
		if !state.Downloaded[i] {
			missing = append(missing, i)
		}
	}
	return missing
}

func chunkName(index int) string {
	return fmt.Sprintf("%d.chunk", index)
}

func jsonMarshal(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", " ")
}