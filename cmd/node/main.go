// Command node runs a sagittarius peer: it can seed a file (--share),
// fetch one (--get), or both, discovering peers through a tracker.
package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/dakshcodez/sagittarius/internal/storage"
	"github.com/dakshcodez/sagittarius/internal/transfer"
)

func main() {
	id := flag.String("id", "", "peer ID (persisted under --storage-dir if omitted)")
	trackerAddr := flag.String("tracker", "127.0.0.1:9090", "tracker address")
	listenAddr := flag.String("listen", "127.0.0.1:9000", "address to listen on, and to advertise to peers")
	storageDir := flag.String("storage-dir", "./data", "directory for chunk storage")
	sharePath := flag.String("share", "", "path to a file to seed")
	getFileID := flag.String("get", "", "file ID to download")
	outDir := flag.String("out", ".", "output directory for --get")
	flag.Parse()

	if *sharePath == "" && *getFileID == "" {
		log.Fatal("node: at least one of --share or --get is required")
	}

	selfID := *id
	if selfID == "" {
		var err error
		selfID, err = loadOrCreateIdentity(*storageDir)
		if err != nil {
			log.Fatalf("node: identity: %v", err)
		}
	}

	st := storage.NewLocalStorage(*storageDir)
	tm := transfer.NewTransferManager(selfID)

	listener, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		log.Fatalf("node: listen on %s failed: %v", *listenAddr, err)
	}
	defer listener.Close()

	go acceptLoop(listener, tm)

	log.Printf("node: peer ID %s listening on %s", selfID, *listenAddr)

	if err := registerWithTracker(*trackerAddr, selfID, *listenAddr); err != nil {
		log.Fatalf("node: register with tracker: %v", err)
	}

	if *sharePath != "" {
		meta, err := seedFile(*sharePath, st, tm, *trackerAddr, selfID, *listenAddr)
		if err != nil {
			log.Fatalf("node: share failed: %v", err)
		}
		log.Printf(
			"node: seeding %q as file ID %s (%d bytes, %d chunks)",
			meta.FileName, meta.FileID, meta.FileSize, meta.NumChunks,
		)
	}

	if *getFileID != "" {
		outputPath, err := downloadFile(*getFileID, *outDir, st, tm, *trackerAddr, selfID)
		if err != nil {
			log.Fatalf("node: get failed: %v", err)
		}
		log.Printf("node: downloaded file %s to %s", *getFileID, outputPath)
	}

	log.Printf("node: serving; press Ctrl+C to stop")
	waitForShutdown()
}

func waitForShutdown() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
}
