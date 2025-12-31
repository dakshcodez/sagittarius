package storage

import (
	"path/filepath"
)

const baseDir = "data/files"

func fileDir(fileID string) string {
	return filepath.Join(baseDir, fileID)
}

func chunksDir(fileID string) string {
	return filepath.Join(fileDir(fileID), "chunks")
}

func metaPath(fileID string) string {
	return filepath.Join(fileDir(fileID), "meta.json")
}

func statePath(fileID string) string {
	return filepath.Join(fileDir(fileID), "state.json")
}