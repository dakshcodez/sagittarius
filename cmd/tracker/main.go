// Command tracker runs the sagittarius rendezvous server: a directory of
// which peers have which files, so nodes can find each other.
package main

import (
	"errors"
	"flag"
	"log"
	"net"

	"github.com/dakshcodez/sagittarius/internal/network"
	"github.com/dakshcodez/sagittarius/internal/trackersrv"
)

func main() {
	addr := flag.String("addr", ":9090", "TCP address for the tracker to listen on (plain-TCP peers/dev convenience)")
	udpAddr := flag.String("udp-addr", ":9090", "UDP address for the tracker's QUIC/NAT-rendezvous listener")
	flag.Parse()

	registry := trackersrv.NewRegistry()
	server := trackersrv.NewServer("tracker", registry)

	done := make(chan struct{})
	registry.StartSweeper(done, trackersrv.DefaultTTL, trackersrv.DefaultSweepInterval)

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("tracker: listen on %s failed: %v", *addr, err)
	}
	defer listener.Close()

	log.Printf("tracker: TCP listening on %s", *addr)

	go serveQUIC(server, *udpAddr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			log.Printf("tracker: accept failed: %v", err)
			continue
		}

		go handleConn(server, conn, trackersrv.RequestContext{})
	}
}

func handleConn(server *trackersrv.Server, raw net.Conn, rc trackersrv.RequestContext) {
	defer raw.Close()

	conn := network.NewConn(raw)

	for {
		msg, err := conn.Receive()
		if err != nil {
			return
		}

		if err := server.HandleMessage(msg, conn, rc); err != nil {
			log.Printf("tracker: error handling %s from %s: %v", msg.Type, msg.SenderID, err)
		}
	}
}
