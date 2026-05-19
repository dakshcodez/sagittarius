package storage_test

import (
	"os"
	"testing"

	"github.com/dakshcodez/sagittarius/internal/filemeta"
	"github.com/dakshcodez/sagittarius/internal/storage"
)

func TestReconstructFile(t *testing.T) {

	st := storage.NewLocalStorage("./testdata")

	meta := &filemeta.FileMeta{
		FileID:    "reconstruct-test",
		FileName:  "hello.txt",
		NumChunks: 3,
	}

	chunks := [][]byte{
		[]byte("hello "),
		[]byte("world"),
		[]byte("!!!"),
	}

	for i, chunk := range chunks {
		err := st.SaveChunk(meta.FileID, i, chunk)
		if err != nil {
			t.Fatal(err)
		}
	}

	outputPath, err := st.ReconstructFile(
		meta,
		"./testdata/output",
	)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	expected := "hello world!!!"

	if string(data) != expected {
		t.Fatalf(
			"expected %q, got %q",
			expected,
			string(data),
		)
	}
}