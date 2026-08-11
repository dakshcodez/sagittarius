// Command node runs a sagittarius peer: it can seed a file (--share),
// fetch one (--get), or both, discovering peers through a tracker either
// over plain TCP or (with --nat) through QUIC/UDP hole punching.
package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/dakshcodez/sagittarius/internal/nat"
	"github.com/dakshcodez/sagittarius/internal/network"
	"github.com/dakshcodez/sagittarius/internal/storage"
	"github.com/dakshcodez/sagittarius/internal/trackersrv"
	"github.com/dakshcodez/sagittarius/internal/transfer"
)

func main() {
	id := flag.String("id", "", "peer ID (persisted under --storage-dir if omitted)")
	trackerAddr := flag.String("tracker", "127.0.0.1:9090", "tracker TCP address")
	listenAddr := flag.String("listen", "127.0.0.1:9000", "TCP address to listen on, and to advertise to peers")
	storageDir := flag.String("storage-dir", "./data", "directory for chunk storage")
	sharePath := flag.String("share", "", "path to a file to seed")
	getFileID := flag.String("get", "", "file ID to download")
	outDir := flag.String("out", ".", "output directory for --get")

	natMode := flag.Bool("nat", false, "reach the tracker and peers over QUIC/UDP with NAT hole punching, instead of plain TCP")
	udpListen := flag.String("udp-listen", ":0", "UDP address to bind for --nat mode")
	trackerUDP := flag.String("tracker-udp", "127.0.0.1:9090", "tracker's QUIC/UDP rendezvous address, for --nat mode")
	forceRelay := flag.Bool("force-relay", false, "skip hole punching and always relay through the tracker (for testing the relay fallback)")

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

	var announce announceFunc
	var lookup lookupFunc
	var connect connectFunc

	if *natMode {
		announce, lookup, connect = setupNAT(*udpListen, *trackerUDP, selfID, tm, *forceRelay)
	} else {
		announce, lookup, connect = setupTCP(*listenAddr, *trackerAddr, selfID, tm)
	}

	if *sharePath != "" {
		meta, err := seedFile(*sharePath, st, tm, announce)
		if err != nil {
			log.Fatalf("node: share failed: %v", err)
		}
		log.Printf(
			"node: seeding %q as file ID %s (%d bytes, %d chunks)",
			meta.FileName, meta.FileID, meta.FileSize, meta.NumChunks,
		)
	}

	if *getFileID != "" {
		outputPath, err := downloadFile(*getFileID, *outDir, st, tm, lookup, connect)
		if err != nil {
			log.Fatalf("node: get failed: %v", err)
		}
		log.Printf("node: downloaded file %s to %s", *getFileID, outputPath)
	}

	log.Printf("node: serving; press Ctrl+C to stop")
	waitForShutdown()
}

// setupTCP brings up the Phase 1 plain-TCP path: a TCP listener plus
// short-lived per-request connections to the tracker.
func setupTCP(listenAddr, trackerAddr, selfID string, tm *transfer.TransferManager) (announceFunc, lookupFunc, connectFunc) {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("node: listen on %s failed: %v", listenAddr, err)
	}

	go acceptLoop(listener, tm)

	log.Printf("node: peer ID %s listening on %s (tcp)", selfID, listenAddr)

	if err := registerWithTracker(trackerAddr, selfID, listenAddr); err != nil {
		log.Fatalf("node: register with tracker: %v", err)
	}

	announce := func(fileID string) error {
		return announceToTracker(trackerAddr, selfID, fileID, listenAddr)
	}
	lookup := func(fileID string) ([]trackersrv.PeerInfo, error) {
		return lookupPeers(trackerAddr, selfID, fileID)
	}
	connect := func(peer trackersrv.PeerInfo) (*network.Conn, string, error) {
		return dialPeer(peer.Addr, selfID)
	}

	return announce, lookup, connect
}

// setupNAT brings up the Phase 2 QUIC/NAT path: one UDP socket used both
// to register with the tracker (learning our reflexive address) and as
// an ambient listener peers can punch through to, plus tracker-brokered
// CONNECT/PUNCH signaling for reaching peers that are behind a NAT.
func setupNAT(udpListen, trackerUDP, selfID string, tm *transfer.TransferManager, forceRelay bool) (announceFunc, lookupFunc, connectFunc) {
	socket, err := nat.Open(udpListen)
	if err != nil {
		log.Fatalf("node: nat socket on %s failed: %v", udpListen, err)
	}

	go natAcceptLoop(socket, tm)

	client, err := dialTrackerNAT(socket, trackerUDP, selfID)
	if err != nil {
		log.Fatalf("node: dial tracker (nat): %v", err)
	}

	localCandidates := nat.GatherLocalCandidates(socket.LocalPort())

	reflexiveAddr, err := client.register(localCandidates)
	if err != nil {
		log.Fatalf("node: nat register: %v", err)
	}

	log.Printf(
		"node: peer ID %s listening on udp port %d (nat), reflexive addr %s, local candidates %v",
		selfID, socket.LocalPort(), reflexiveAddr, localCandidates,
	)

	go client.keepalive(localCandidates)

	go client.runPushListener(
		func(peerID string, candidates []string) {
			log.Printf("node: punch signal from %s, candidates %v", peerID, candidates)
			nat.PunchCandidates(socket, candidates, nat.DefaultPunchTimeout)
		},
		func(raw net.Conn) {
			log.Printf("node: accepted relayed connection via tracker")
			handleIncoming(raw, tm)
		},
	)

	announce := client.announce
	lookup := client.lookup
	connect := func(peer trackersrv.PeerInfo) (*network.Conn, string, error) {
		var conn *network.Conn
		var peerID, via string
		var err error

		if forceRelay {
			conn, peerID, via, err = natRelayToPeer(client, selfID, peer.PeerID)
		} else {
			conn, peerID, via, err = natConnectToPeer(client, socket, selfID, peer.PeerID, nat.DefaultPunchTimeout)
		}

		if err == nil {
			log.Printf("node: connected to %s via %s", peerID, via)
		}
		return conn, peerID, err
	}

	return announce, lookup, connect
}

func waitForShutdown() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
}
