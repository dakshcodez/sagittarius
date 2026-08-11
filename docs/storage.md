# Storage

`internal/storage.LocalStorage` is the on-disk chunk store, and the only
implementation of `transfer.Storage` transfer sessions actually use.

## Layout

```
<base-dir>/<file-id>/meta.json          FileMeta (see docs/file-format.md)
<base-dir>/<file-id>/chunks/<n>.chunk   raw bytes of chunk n
```

`SaveChunk` writes to a `.tmp` file and renames it into place, so a chunk
is either fully present or not present at all - no partial-chunk state to
detect or clean up after a crash mid-write.

## Resume support

There's deliberately no separate "downloaded" bitmap/state file: a
chunk's presence *is* its download state.
`GetMissingChunks`/`HasChunk` check the filesystem directly, so resuming
after a restart is just constructing a new `LocalStorage` over the same
directory and a new `DownloadSession` from the same `FileMeta` - it
reads storage truth and picks up exactly where it left off
(`internal/transfer/session.go`'s `NewDownloadSession`, exercised by
`internal/storage/storage_test.go`'s `TestResumeAfterRestart`).

## Seeding

`filemeta.CreateFileMeta` only hashes/sizes chunks - it doesn't retain
their bytes. `cmd/node/share.go`'s `copyChunksIntoStorage` re-reads the
source file and calls `SaveChunk` per chunk to actually populate storage
before a `DownloadSession` can be built with every chunk already
`ChunkComplete` (i.e. a seeder).
