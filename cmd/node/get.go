package main

import (
	"fmt"
	"log"
	"time"

	"github.com/dakshcodez/sagittarius/internal/filemeta"
	"github.com/dakshcodez/sagittarius/internal/storage"
	"github.com/dakshcodez/sagittarius/internal/transfer"
)

const (
	metaWaitTimeout     = 15 * time.Second
	downloadTimeout     = 10 * time.Minute
	pollInterval        = 200 * time.Millisecond
	progressLogInterval = 2 * time.Second
)

// downloadFile looks up peers for fileID via the tracker, connects to one,
// fetches its metadata, downloads every chunk, and reconstructs the file
// into outDir. On success it leaves the download session registered with
// tm so this node can go on to seed the file to others.
func downloadFile(
	fileID, outDir string,
	st *storage.LocalStorage,
	tm *transfer.TransferManager,
	trackerAddr, selfID string,
) (string, error) {

	peers, err := lookupPeers(trackerAddr, selfID, fileID)
	if err != nil {
		return "", fmt.Errorf("lookup %s: %w", fileID, err)
	}
	if len(peers) == 0 {
		return "", fmt.Errorf("no peers advertise file %s", fileID)
	}

	var lastErr error
	for _, peer := range peers {
		path, err := downloadFrom(peer.Addr, fileID, outDir, st, tm, selfID)
		if err == nil {
			return path, nil
		}
		log.Printf("node: download from %s (%s) failed: %v", peer.PeerID, peer.Addr, err)
		lastErr = err
	}

	return "", fmt.Errorf("all peers failed, last error: %w", lastErr)
}

func downloadFrom(
	peerAddr, fileID, outDir string,
	st *storage.LocalStorage,
	tm *transfer.TransferManager,
	selfID string,
) (string, error) {

	conn, peerID, err := dialPeer(peerAddr, selfID)
	if err != nil {
		return "", fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()

	go messageLoop(conn, tm, peerID)

	if err := tm.RequestMeta(fileID, conn); err != nil {
		return "", fmt.Errorf("request meta: %w", err)
	}

	meta, err := waitForMeta(tm, fileID, metaWaitTimeout)
	if err != nil {
		return "", err
	}

	if err := st.InitFileStorage(meta); err != nil {
		return "", fmt.Errorf("init storage: %w", err)
	}

	session := transfer.NewDownloadSession(meta, st)
	tm.AddSession(session)
	session.StartDownload(conn)

	if err := waitForCompletion(session, downloadTimeout); err != nil {
		return "", err
	}

	outputPath, err := st.ReconstructFile(meta, outDir)
	if err != nil {
		return "", fmt.Errorf("reconstruct: %w", err)
	}

	return outputPath, nil
}

func waitForMeta(tm *transfer.TransferManager, fileID string, timeout time.Duration) (*filemeta.FileMeta, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if meta, ok := tm.GetMeta(fileID); ok {
			return meta, nil
		}
		time.Sleep(pollInterval)
	}

	return nil, fmt.Errorf("timed out waiting for metadata of %s", fileID)
}

func waitForCompletion(session *transfer.DownloadSession, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	lastLog := time.Now()

	for time.Now().Before(deadline) {
		if session.IsComplete() {
			return nil
		}

		if time.Since(lastLog) >= progressLogInterval {
			log.Printf("node: download in progress...")
			lastLog = time.Now()
		}

		time.Sleep(pollInterval)
	}

	return fmt.Errorf("timed out after %s waiting for download to complete", timeout)
}
