# Wire Protocol

## Framing

Every message is a 4-byte big-endian length prefix followed by that many
bytes of JSON (`internal/network/framing.go`). Frame length is capped at
`network.MaxFrameSize` (16MiB) so a corrupt/hostile length prefix can't
force an unbounded allocation.

## Envelope

```json
{"type": "...", "sender_id": "...", "payload": { ... }}
```

`internal/network.Message`. `payload` is message-type-specific JSON,
decoded on demand by whichever layer handles that type.

## Peer <-> peer messages (`internal/network`, `internal/transfer`)

Sent directly between two connected peers, over whichever transport
connected them (plain TCP, a direct/punched QUIC stream, or a relayed
QUIC stream - identical either way).

| Type | Payload | Purpose |
|---|---|---|
| `HANDSHAKE` | `{protocol_version}` | Exchanged first in both directions (initiator sends-then-receives, responder receives-then-sends) to confirm protocol compatibility and learn the peer's ID. |
| `META_REQUEST` | `{file_id}` | Ask for a file's chunk metadata. |
| `META_RESPONSE` | `{meta: FileMeta}` | The chunk list (index, hash, size) needed to verify and reconstruct the file. |
| `CHUNK_REQUEST` | `{file_id, chunk_index}` | Ask for one chunk's bytes. |
| `CHUNK_RESPONSE` | `{file_id, chunk_index, data}` | The chunk's bytes (base64 via JSON). Verified against the hash in `FileMeta` before being saved. |

`DownloadSession` (`internal/transfer/session.go`) pipelines up to 8
`CHUNK_REQUEST`s in flight at once rather than waiting for each response,
and a watchdog re-requests any chunk that doesn't answer within 5s.

## Node <-> tracker messages (`internal/trackersrv`)

| Type | Payload | Purpose |
|---|---|---|
| `REGISTER` | `{peer_id, addr, local_candidates}` | Announce presence. Over TCP, `addr` is the peer's own claimed dialable address. Over QUIC, the tracker ignores `addr` and instead observes the connection's remote address as a STUN-style reflexive address, returned in the ack. |
| `REGISTER_ACK` | `{ok, reflexive_addr}` | |
| `ANNOUNCE` | `{peer_id, file_id, addr}` | "I have this file." |
| `ANNOUNCE_ACK` | `{ok}` | |
| `LOOKUP` | `{file_id}` | Find peers advertising a file. |
| `PEER_LIST` | `{peers: [{peer_id, addr}]}` | |
| `CONNECT` | `{target_peer_id}` | Ask the tracker to broker a connection to another registered peer. |
| `PUNCH` | `{peer_id, candidates}` | Sent two ways: as the direct reply to `CONNECT` (candidates = the target's), and pushed unprompted to the target over its already-open control connection (candidates = the requester's). Both sides then race dials against the candidates they received. |
| `RELAY` | `{target_peer_id}` | Sent as the *first* message on a stream that should become a raw relayed pipe to another peer. Everything after it on that stream is opaque P2P traffic the tracker splices through, not JSON it parses. Also used (with the sender/target roles swapped) as the header on the stream the tracker opens on the target's side. |

QUIC-registered peers keep one persistent control connection to the
tracker; each request opens a fresh stream on it (cheap - no new QUIC
handshake), and the tracker can independently open streams back at the
peer at any time (used for `PUNCH` pushes and `RELAY` invitations).
Plain-TCP requests instead open a short-lived TCP connection per request.

See `docs/tracker.md` for the full CONNECT/PUNCH/RELAY sequence, and
`docs/file-format.md` for `FileMeta`'s content-addressing scheme.
