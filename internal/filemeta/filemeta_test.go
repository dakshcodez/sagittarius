package filemeta

import (
	"os"
	"testing"
)

func TestCreateFileMeta(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "chunk-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	data := []byte("this is a test file for chunking")
	_, err = tmpFile.Write(data)
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	meta, err := CreateFileMeta(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	if meta.NumChunks == 0 {
		t.Fatal("expected chunks, got zero")
	}

	if meta.FileID == "" {
		t.Fatal("file ID should not be empty")
	}
}