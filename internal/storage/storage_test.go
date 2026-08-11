package storage_test

import (
	"testing"

	"github.com/dakshcodez/sagittarius/internal/filemeta"
	"github.com/dakshcodez/sagittarius/internal/storage"
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
	st := storage.NewLocalStorage(t.TempDir())

	if err := st.InitFileStorage(meta); err != nil {
		t.Fatalf("InitFileStorage failed: %v", err)
	}

	loaded, err := st.LoadFileMeta(meta.FileID)
	if err != nil {
		t.Fatalf("LoadFileMeta failed: %v", err)
	}

	if loaded.FileID != meta.FileID || loaded.NumChunks != meta.NumChunks {
		t.Fatalf("loaded meta does not match: %+v", loaded)
	}
}

func TestSaveAndHasChunk(t *testing.T) {
	meta := testMeta()
	st := storage.NewLocalStorage(t.TempDir())

	if err := st.InitFileStorage(meta); err != nil {
		t.Fatal(err)
	}

	data := []byte("chunk-data")

	if err := st.SaveChunk(meta.FileID, 1, data); err != nil {
		t.Fatalf("SaveChunk failed: %v", err)
	}

	if !st.HasChunk(meta.FileID, 1) {
		t.Fatal("HasChunk returned false for saved chunk")
	}
}

func TestLoadChunk(t *testing.T) {
	meta := testMeta()
	st := storage.NewLocalStorage(t.TempDir())

	if err := st.InitFileStorage(meta); err != nil {
		t.Fatal(err)
	}

	expected := []byte("hello")
	if err := st.SaveChunk(meta.FileID, 0, expected); err != nil {
		t.Fatal(err)
	}

	data, err := st.LoadChunk(meta.FileID, 0)
	if err != nil {
		t.Fatalf("LoadChunk failed: %v", err)
	}

	if string(data) != string(expected) {
		t.Fatalf("chunk data mismatch")
	}
}

func TestGetMissingChunks(t *testing.T) {
	meta := testMeta()
	st := storage.NewLocalStorage(t.TempDir())

	if err := st.InitFileStorage(meta); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveChunk(meta.FileID, 0, []byte("a")); err != nil {
		t.Fatal(err)
	}

	missing := st.GetMissingChunks(meta)

	if len(missing) != 2 {
		t.Fatalf("expected 2 missing chunks, got %d", len(missing))
	}
}

func TestResumeAfterRestart(t *testing.T) {
	meta := testMeta()
	dir := t.TempDir()
	st := storage.NewLocalStorage(dir)

	// first "run"
	if err := st.InitFileStorage(meta); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveChunk(meta.FileID, 1, []byte("data")); err != nil {
		t.Fatal(err)
	}

	// simulate restart: fresh LocalStorage over the same directory, and
	// reload meta from disk the way a real restart would.
	restarted := storage.NewLocalStorage(dir)

	reloadedMeta, err := restarted.LoadFileMeta(meta.FileID)
	if err != nil {
		t.Fatalf("LoadFileMeta failed after restart: %v", err)
	}

	missing := restarted.GetMissingChunks(reloadedMeta)

	for _, idx := range missing {
		if idx == 1 {
			t.Fatal("downloaded chunk incorrectly marked missing")
		}
	}
}
