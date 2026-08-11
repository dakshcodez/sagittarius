package nat

import (
	"context"
	"fmt"
	"time"

	"github.com/quic-go/quic-go"
)

// DefaultPunchTimeout bounds how long DialCandidates waits across all
// candidates before giving up (and callers falling back to a relay).
const DefaultPunchTimeout = 5 * time.Second

type dialResult struct {
	addr string
	conn *quic.Conn
	err  error
}

// DialCandidates races concurrent dials to every candidate address (LAN
// host candidates and the tracker-observed public/reflexive candidate
// alike) and returns the first that succeeds, along with the address that
// won so callers can log whether the connection ended up direct (LAN) or
// punched (WAN).
//
// Firing all candidates at once rather than trying them in priority order
// is deliberate: on a NATed path, the dial itself is what opens the local
// NAT's outbound mapping, so there is no "cheap" candidate to try first -
// every attempt has to be made regardless, and racing them minimizes
// total wait time.
func DialCandidates(ctx context.Context, sock *Socket, candidates []string, timeout time.Duration) (*quic.Conn, string, error) {
	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("no candidates to dial")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	results := make(chan dialResult, len(candidates))

	for _, addr := range candidates {
		go func(addr string) {
			conn, err := sock.Dial(ctx, addr)
			results <- dialResult{addr: addr, conn: conn, err: err}
		}(addr)
	}

	var lastErr error
	for range candidates {
		select {
		case r := <-results:
			if r.err == nil {
				return r.conn, r.addr, nil
			}
			lastErr = r.err
		case <-ctx.Done():
			return nil, "", fmt.Errorf("punch timed out after %s: %w", timeout, ctx.Err())
		}
	}

	return nil, "", fmt.Errorf("all %d candidate(s) failed, last error: %w", len(candidates), lastErr)
}

// PunchCandidates is the target-side half of a hole punch: it fires
// best-effort concurrent dials at the requester's candidates purely to
// open this node's own outbound NAT mapping toward them. Any connection
// that happens to succeed is closed immediately - the actual data
// connection is expected to arrive via the socket's ambient Accept loop
// once the requester's dial gets through. Errors are intentionally
// swallowed: this is a best-effort assist, not something the caller
// blocks on.
func PunchCandidates(sock *Socket, candidates []string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, addr := range candidates {
		go func(addr string) {
			conn, err := sock.Dial(ctx, addr)
			if err == nil {
				conn.CloseWithError(0, "punch")
			}
		}(addr)
	}

	<-ctx.Done()
}
