package transfer_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"testing"
	"time"

	"github.com/dakshcodez/sagittarius/internal/filemeta"
	"github.com/dakshcodez/sagittarius/internal/network"
	"github.com/dakshcodez/sagittarius/internal/storage"
	"github.com/dakshcodez/sagittarius/internal/transfer"
)

func TestChunkTransfer(t *testing.T) {

	// Create in-memory TCP connection pair
	conn1, conn2 := net.Pipe()

	seederConn := network.NewConn(conn1)
	downloaderConn := network.NewConn(conn2)

	chunkData := []byte("hello world")

	sum := sha256.Sum256(chunkData)
	hash := hex.EncodeToString(sum[:])

	// Create metadata
	meta := &filemeta.FileMeta{
		FileID:    "test-file",
		FileName:  "hello.txt",
		FileSize:  11,
		ChunkSize: 5,
		NumChunks: 1,
		Chunks: []filemeta.ChunkMeta{
			{
				Index: 0,
				Hash:  hash,
			},
		},
	}

	// Create storage
	seederStorage := storage.NewLocalStorage("./testdata/seeder")
	downloaderStorage := storage.NewLocalStorage("./testdata/downloader")

	// Save chunk into seeder storage
	err := seederStorage.SaveChunk(
		meta.FileID,
		0,
		chunkData,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Create sessions
	seederSession := transfer.NewDownloadSession(meta, seederStorage)
	downloaderSession := transfer.NewDownloadSession(meta, downloaderStorage)

	// Create managers
	seederTM := transfer.NewTransferManager("seeder")
	downloaderTM := transfer.NewTransferManager("downloader")

	seederTM.AddSession(seederSession)
	downloaderTM.AddSession(downloaderSession)

	// Seeder receive loop
	go func() {
		for {
			msg, err := seederConn.Receive()
			if err != nil {
				return
			}

			_ = seederTM.HandleNetworkMessage(msg, seederConn)
		}
	}()

	// Downloader receive loop
	go func() {
		for {
			msg, err := downloaderConn.Receive()
			if err != nil {
				return
			}

			_ = downloaderTM.HandleNetworkMessage(msg, downloaderConn)
		}
	}()

	// Start download
	downloaderSession.StartDownload(downloaderConn)

	// Give transfer time
	time.Sleep(2 * time.Second)

	// Verify downloader now has chunk
	if !downloaderStorage.HasChunk(meta.FileID, 0) {
		t.Fatal("chunk was not downloaded")
	}
}