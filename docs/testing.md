# Testing

## Automated

```
go build ./...
go vet ./...
go test ./...
```

Every package has unit tests except `cmd/node`/`cmd/tracker` themselves
(thin CLI wiring - covered by the manual end-to-end flows below instead).
Notable ones:

- `internal/nat`: a real loopback QUIC dial/accept/stream round-trip
  (`TestDialCandidatesLoopback`), proving the "one socket both dials and
  listens" mechanism NAT traversal depends on actually works.
- `internal/trackersrv`: `TestConnectSignalsPunchBothWays` exercises the
  full CONNECT -> PUNCH (reply) + PUNCH (pushed) sequence against a fake
  `Pusher`.
- `internal/transfer`: `TestNextRequestableRespectsWindow` and
  `TestRequeueStaleRequests*` cover the request-pipelining window and
  stale-request retry logic directly, without needing to wait out real
  timeouts.

## Manual end-to-end

These are the flows actually used to validate this system while it was
built - useful as a smoke test after any change to `cmd/node`,
`cmd/tracker`, or the transport layers.

Build once:

```
go build -o /tmp/sag/tracker ./cmd/tracker
go build -o /tmp/sag/node ./cmd/node
```

### Plain TCP (same host/LAN)

```
./tracker --addr 127.0.0.1:9090

./node --tracker 127.0.0.1:9090 --listen 127.0.0.1:9001 \
  --storage-dir ./seeder-data --share /path/to/file
# note the printed file ID

./node --tracker 127.0.0.1:9090 --listen 127.0.0.1:9002 \
  --storage-dir ./getter-data --get <file-id> --out ./downloaded
```

### QUIC/NAT (still localhost, but exercises the real punch/QUIC path)

```
./tracker --addr 127.0.0.1:9090 --udp-addr 127.0.0.1:9090

./node --nat --tracker-udp 127.0.0.1:9090 --udp-listen 127.0.0.1:9101 \
  --storage-dir ./seeder-data --share /path/to/file

./node --nat --tracker-udp 127.0.0.1:9090 --udp-listen 127.0.0.1:9102 \
  --storage-dir ./getter-data --get <file-id> --out ./downloaded
```

Watch for `node: connected to <peer> via <addr>` - the address confirms a
real QUIC connection was established, not just the tracker signaling.

### Relay fallback (no symmetric NAT required)

Add `--force-relay` to the getter's command above. The tracker log should
show `tracker: relaying <A> <-> <B>`, and the download should still
complete and reconstruct correctly.

### Real cross-network NAT traversal

Run the tracker on a reachable host (public IP, or one forwarded UDP
port) and the two nodes on genuinely different networks. See
`docs/tracker.md` for what's actually happening on the wire. This is the
one flow that can't be verified from a single machine - loopback proves
the mechanism is correct, but not that it survives real NATs.

### Verifying correctness

After any of the above, compare hashes:

```
sha256sum /path/to/file ./downloaded/<filename>
```

They must match exactly - `DownloadSession.verifyChunk` already checks
each chunk's hash against `FileMeta` before saving it, so a mismatch here
would mean a bug in reconstruction, not transfer.
