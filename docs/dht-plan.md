# DHT: not built, and why

Sagittarius uses a centralized tracker (`internal/trackersrv`,
`cmd/tracker`) for peer discovery and NAT-traversal signaling, not a
distributed hash table. That was a deliberate choice for the prototype,
not a placeholder for one that got skipped by accident - a Kademlia-style
DHT (as used by trackerless BitTorrent/mainline DHT) solves peer
discovery *without* a central server, but it does not solve NAT
traversal, which was the actual hard requirement here. A DHT still needs
*something* to hand two NATed peers each other's address candidates and
help coordinate a punch; that something ends up looking a lot like the
tracker's `CONNECT`/`PUNCH` signaling (`docs/tracker.md`) either way. A
centralized tracker gets that signaling working with one predictable,
always-reachable rendezvous point, which is what made a working
NAT-traversal prototype realistic to build and to actually test.

## If a DHT were added later

The pieces are shaped so this would be additive rather than a rewrite:

- `trackersrv.Registry`'s `LOOKUP`/`ANNOUNCE` (peer_id -> file_id ->
  address candidates) is exactly the query shape a DHT `get_peers` would
  need to answer instead - it could be backed by a Kademlia routing table
  instead of an in-memory map without changing `internal/transfer` or
  `internal/nat` at all, since both only depend on getting *some* list of
  candidate addresses back.
- NAT-traversal signaling (`CONNECT`/`PUNCH`/`RELAY`) would still need at
  least one reachable node to broker the initial punch and relay for
  symmetric-NAT pairs - in a DHT world this is usually a subset of
  well-connected peers acting as rendezvous/relay nodes rather than one
  dedicated tracker process, but the protocol itself
  (`docs/protocol.md`) wouldn't need to change, just who's allowed to
  speak it.
- The single-tracker design is also the current scaling/availability
  limit worth flagging: if the tracker process is down, no new peers can
  find each other (existing direct connections keep working). A DHT - or
  even just running multiple independent tracker instances peers can
  fall back between - is the natural next step if that becomes a real
  constraint.
