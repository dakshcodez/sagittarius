package main

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// loadOrCreateIdentity returns a stable peer ID for this storage directory,
// generating and persisting a new random one on first run.
func loadOrCreateIdentity(storageDir string) (string, error) {
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return "", err
	}

	idPath := filepath.Join(storageDir, "node_id")

	data, err := os.ReadFile(idPath)
	if err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id, nil
		}
	}

	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	id := hex.EncodeToString(buf)

	if err := os.WriteFile(idPath, []byte(id), 0644); err != nil {
		return "", err
	}

	return id, nil
}
