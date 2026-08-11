# Tracker

The tracker (`cmd/tracker`) is a centralized rendezvous server: it knows
which peers have which files and, for peers behind a NAT, helps them
establish a direct connection (or relays one). It holds no chunk data
itself.

It runs two listeners over the same in-memory `trackersrv.Registry`:

- **TCP** (`--addr`, default `:9090`): plain-TCP peers' short-lived,
  one-request-per-connection registration/announce/lookup.
- **QUIC/UDP** (`--udp-addr`, default `:9090`): `--nat` peers' persistent
  control connections, used for registration (doubling as a STUN
  responder - see below), lookup, and NAT-traversal signaling.

Peers are evicted (`Registry.Sweep`, `docs/protocol.md`'s `REGISTER` TTL)
after 90s of no contact; `--nat` nodes send a keepalive `REGISTER` every
25s to stay registered and keep their NAT mapping open.

## Reflexive address discovery (STUN, without a STUN server)

When a `--nat` node dials the tracker over QUIC, the tracker's accepted
connection's `RemoteAddr()` *is* that peer's public, NAT-mapped address -
exactly what a STUN binding response would tell you. The tracker just
returns it in the `REGISTER_ACK`, no separate STUN protocol/server
needed.

## Connecting two peers (CONNECT / PUNCH)

```
   A                          tracker                         B
   |--- CONNECT{target: B} -->|                                |
   |                          |--- PUNCH{peer: A, cands} ----->|
   |<-- PUNCH{peer: B, cands}-|                                |
   |                                                            |
   |======= both sides race concurrent QUIC dials  ============|
   |            at every candidate they were given             |
```

A's dial to one of B's candidates succeeding *is* the connection - B's
own ambient QUIC listener (always running) surfaces the matching side of
that same handshake independently, so B doesn't need any special
"pending connect" state. B also fires its own best-effort dials at A's
candidates purely to open its own NAT mapping toward A
(`nat.PunchCandidates`) - that best-effort attempt is what makes punching
work in the first place when both sides are NATed, even though B doesn't
need to keep whatever connection object it produces.

## Relay fallback

If every candidate dial fails within the timeout (typically: both peers
behind symmetric/carrier-grade NAT), the requester instead sends `RELAY`:

```
   A                          tracker                         B
   |--- RELAY{target: B} ---->|                                |
   |   (stream now paused,    |--- RELAY{peer: A} (new stream)>|
   |    waiting to be spliced)|                                |
   |                          |<====== tracker splices ======>|
   |<========================== raw bytes both ways ==========>|
```

The tracker treats A's `RELAY` stream and the new stream it opens on B's
control connection as an opaque pipe (`io.Copy` both directions) from
that point on - it never looks at what flows through it again. Both A and
B run their normal handshake and P2P protocol over their respective end,
unaware it's relayed rather than direct.

`cmd/node --force-relay` skips punching and always takes this path -
useful for testing without needing genuine symmetric NAT.
