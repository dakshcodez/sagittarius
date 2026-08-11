package nat_test

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/dakshcodez/sagittarius/internal/nat"
	"github.com/dakshcodez/sagittarius/internal/quicconn"
)

// TestDialCandidatesLoopback exercises the full quic-go Transport
// dial/listen-on-one-socket mechanism the punch design depends on: two
// sockets, each with its own ambient listener, where one dials the other
// and a byte round-trips over the resulting stream via the quicconn
// net.Conn adapter.
func TestDialCandidatesLoopback(t *testing.T) {
	sockA, err := nat.Open("127.0.0.1:0")
	if err != nil {
		t.Fatalf("open socket A: %v", err)
	}
	defer sockA.Close()

	sockB, err := nat.Open("127.0.0.1:0")
	if err != nil {
		t.Fatalf("open socket B: %v", err)
	}
	defer sockB.Close()

	bAddr := fmt.Sprintf("127.0.0.1:%d", sockB.LocalPort())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// A dials B directly (localhost stand-in for "candidate address").
	qconnA, winningAddr, err := nat.DialCandidates(ctx, sockA, []string{bAddr}, 3*time.Second)
	if err != nil {
		t.Fatalf("DialCandidates failed: %v", err)
	}
	if winningAddr != bAddr {
		t.Fatalf("expected winning addr %s, got %s", bAddr, winningAddr)
	}

	// B's ambient listener should surface the same handshake.
	qconnB, err := sockB.Accept(ctx)
	if err != nil {
		t.Fatalf("socket B accept: %v", err)
	}

	go func() {
		streamA, err := quicconn.OpenStream(qconnA)
		if err != nil {
			t.Errorf("open stream: %v", err)
			return
		}
		streamA.Write([]byte("hello over quic"))
		streamA.Close()
	}()

	streamB, err := quicconn.AcceptStream(qconnB)
	if err != nil {
		t.Fatalf("accept stream: %v", err)
	}

	data, err := io.ReadAll(streamB)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if string(data) != "hello over quic" {
		t.Fatalf("unexpected payload: %q", data)
	}
}

func TestGatherLocalCandidatesSkipsLoopback(t *testing.T) {
	candidates := nat.GatherLocalCandidates(9000)
	for _, c := range candidates {
		if c == "127.0.0.1:9000" {
			t.Fatalf("loopback candidate should have been excluded, got %v", candidates)
		}
	}
}
