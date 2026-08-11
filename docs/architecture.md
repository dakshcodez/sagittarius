# Architecture

Sagittarius is a BitTorrent-style P2P file transfer system: files are
content-addressed by chunk hash, peers exchange chunks directly over a
connection, and a tracker helps peers find each other.

## Packages

```
internal/filemeta   chunking + hashing (content addressing)
internal/storage     on-disk chunk storage, resumable by construction
internal/network     wire framing, the Message envelope, handshake
internal/transfer     download session state machine + chunk request/response
internal/trackersrv   tracker's peer/file registry and protocol dispatch
internal/nat          UDP socket + QUIC transport, candidate gathering, hole punching
internal/quicconn     adapts a QUIC stream to net.Conn
cmd/tracker            tracker binary (TCP + QUIC listeners)
cmd/node                peer binary (--share / --get, plain-TCP or --nat)
```

`internal/network`, `internal/transfer`, and `internal/trackersrv` only
ever depend on `net.Conn`-shaped interfaces (`network.Conn` wraps
`net.Conn` directly; `transfer.NetworkSender` is just `Send(msg any)
error`). None of them know or care whether the underlying pipe is a plain
TCP connection, a QUIC stream, or a stream relayed through the tracker -
that's what let NAT traversal and relay get added later without touching
the download/reconstruct logic at all.

## Two ways to reach a peer

**Plain TCP** (`cmd/node` without `--nat`): each node runs a TCP listener
and registers its dialable address with the tracker over short-lived TCP
requests. Works when peers are on the same host/LAN or already reachable
(public IP, port forward). Simple, no NAT traversal.

**QUIC/NAT** (`cmd/node --nat`): each node opens one UDP socket wrapped in
a `quic.Transport` (`internal/nat`), which can both dial out and listen
for inbound connections *on the same socket* - `quic-go` multiplexes by
QUIC connection ID rather than by 4-tuple, so this works. That socket is
used to:

1. Register with the tracker. The tracker observes the QUIC connection's
   remote address and hands it back - a STUN-style reflexive address,
   piggybacked on registration instead of needing a separate STUN server.
2. Reach another peer via a `CONNECT` request: the tracker replies with
   that peer's candidates (its LAN addresses + reflexive address) and, at
   the same time, pushes *our* candidates to them. Both sides then race
   concurrent QUIC dials against every candidate (`nat.DialCandidates`) -
   this is what actually punches the NAT hole. See `docs/protocol.md` for
   the exact message sequence.
3. Fall back to a relay if punching fails (e.g. both peers behind
   symmetric NAT): the tracker splices two QUIC streams together and
   proxies raw bytes between them, so the P2P protocol running over that
   pipe can't tell it wasn't a direct connection.

## A node's lifecycle

1. Load or generate a stable peer ID (`cmd/node/identity.go`, persisted
   under `--storage-dir/node_id`).
2. Bring up whichever transport (`setupTCP` / `setupNAT` in
   `cmd/node/main.go`) and register with the tracker.
3. `--share`: hash/chunk the file (`filemeta.CreateFileMeta`), copy chunk
   data into storage, register a `DownloadSession` that's already 100%
   complete (i.e. a seeder), and `ANNOUNCE` it.
4. `--get`: `LOOKUP` peers for the file ID, connect to one, `RequestMeta`,
   then `DownloadSession.StartDownload` pipelines chunk requests
   (`internal/transfer/session.go`) until every chunk is verified
   (SHA-256 against the metadata) and saved, then reconstruct the file.
5. Either way, the node keeps serving afterward - a completed download
   becomes a seed for other peers, same as BitTorrent.
