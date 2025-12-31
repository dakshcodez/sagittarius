package storage

import (
	"os"
	//"path/filepath"
	"testing"

	"github.com/dakshcodez/sagittarius/internal/filemeta"
)

// helper: create fake FileMeta
func testMeta() *filemeta.FileMeta {
	return &filemeta.FileMeta{
		FileID:    "test-file-id",
		FileName:  "test.txt",
		FileSize:  10,
		ChunkSize: 4,
		NumChunks: 3,
		Chunks: []filemeta.ChunkMeta{
			{Index: 0, Hash: "a", Size: 4},
			{Index: 1, Hash: "b", Size: 4},
			{Index: 2, Hash: "c", Size: 2},
		},
	}
}

func TestInitFileStorage(t *testing.T) {
	meta := testMeta()

	// clean before test
	os.RemoveAll(baseDir)
	defer os.RemoveAll(baseDir)

	if err := InitFileStorage(meta); err != nil {
		t.Fatalf("InitFileStorage failed: %v", err)
	}

	if _, err := os.Stat(metaPath(meta.FileID)); err != nil {
		t.Fatal("meta.json was not created")
	}

	if _, err := os.Stat(statePath(meta.FileID)); err != nil {
		t.Fatal("state.json was not created")
	}
}

func TestSaveAndHasChunk(t *testing.T) {
	meta := testMeta()
	os.RemoveAll(baseDir)
	defer os.RemoveAll(baseDir)

	InitFileStorage(meta)

	data := []byte("chunk-data")

	if err := SaveChunk(meta.FileID, 1, data); err != nil {
		t.Fatalf("SaveChunk failed: %v", err)
	}

	if !HasChunk(meta.FileID, 1) {
		t.Fatal("HasChunk returned false for saved chunk")
	}
}

func TestLoadChunk(t *testing.T) {
	meta := testMeta()
	os.RemoveAll(baseDir)
	defer os.RemoveAll(baseDir)

	InitFileStorage(meta)

	expected := []byte("hello")
	SaveChunk(meta.FileID, 0, expected)

	data, err := LoadChunk(meta.FileID, 0)
	if err != nil {
		t.Fatalf("LoadChunk failed: %v", err)
	}

	if string(data) != string(expected) {
		t.Fatalf("chunk data mismatch")
	}
}

func TestGetMissingChunks(t *testing.T) {
	meta := testMeta()
	os.RemoveAll(baseDir)
	defer os.RemoveAll(baseDir)

	InitFileStorage(meta)
	SaveChunk(meta.FileID, 0, []byte("a"))

	missing := GetMissingChunks(meta)

	if len(missing) != 2 {
		t.Fatalf("expected 2 missing chunks, got %d", len(missing))
	}
}

func TestResumeAfterRestart(t *testing.T) {
	meta := testMeta()
	os.RemoveAll(baseDir)
	defer os.RemoveAll(baseDir)

	// first "run"
	InitFileStorage(meta)
	SaveChunk(meta.FileID, 1, []byte("data"))

	// simulate restart by reloading state
	missing := GetMissingChunks(meta)

	for _, idx := range missing {
		if idx == 1 {
			t.Fatal("downloaded chunk incorrectly marked missing")
		}
	}
}
