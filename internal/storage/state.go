package storage

import (
	"encoding/json"
	"os"
)

type FileState struct {
	Downloaded map[int]bool `json:"downloaded"`
}

func loadState(path string) (*FileState, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &FileState{Downloaded: make(map[int]bool)}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var state FileState
	err = json.Unmarshal(data, &state)
	return &state, err
}

func saveState(path string, state *FileState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}