# Result stream compression

Proxied query results go over the wire as Arrow IPC compressed with zstd
(`Sink` → `Content-Encoding: zstd`, decoded by `Stream`). Arrow IPC is
uncompressed columnar layout, so it is bulky and repetitive and compresses
very well: 12x on a typical 50k-row table, 19x on a numeric scan, 51x on a log
table, ~1.4x on already-random data (uuids, hashes, ciphertext).

That is worth 3-10x on time-to-last-row for anyone not sitting in the
datacenter — a 50k-row pull is 153 ms rather than 933 ms on a 50 Mbps home
link, 475 ms rather than 4.6 s on cafe wifi.

## Codec pooling

Both codecs are pooled (`encoderPool` in `sink.go`, `decoderPool` in
`stream.go`). Constructing a zstd codec costs far more than running it on a
typical result set: a fresh encoder is ~19 MB of allocation and several ms of
CPU before the first row is compressed, which a 500-row grid page has nothing
to amortise against.

The contract, which the tests in `pooling_test.go` pin down:

- **A codec is returned to its pool only by `Close`.** `Sink.Close` closes the
  frame, then `Reset(nil)` to drop the reference to the response writer;
  `Stream.Close` uses `Reset(nil)` rather than `Close`, which would retire the
  decoder permanently.
- **`Close` is idempotent on both.** A double `Close` must never park the same
  codec twice — two concurrent queries sharing one encoder would interleave
  their bytes.
- **Reset purges history.** `Encoder.Reset` clears the match window and offsets
  prior history out of reach, so no bytes and no back-references cross from one
  query's stream into another's. This holds even when a stream dies mid-flight:
  a broken stream's encoder comes back clean.
- **Pooled, not shared.** A `zstd.Decoder` decodes one *stream* at a time. Only
  its stateless `DecodeAll` is concurrency-safe, which is why the JSON path in
  `app/internal/api/fetch.go` can share one package-level decoder and this path
  cannot.

## Tuning

- **Window: 1 MiB** (`compressionWindow`), against the library default of
  8 MiB. The window is live memory for the whole life of a stream. 1 MiB keeps
  85-95% of the ratio (12.4x → 11.1x on a 50k-row table, 51x → 48x on logs) for
  roughly a 5x cut in per-stream memory. Below 1 MiB the ratio falls off
  sharply for numeric and log-shaped data.
- **Concurrency: 1.** Compression runs on the calling goroutine. A single query
  can no longer fan its own compression across every core — which is what we
  want on a shared proxy — and each stream drops its worker goroutines and
  their block buffers.
- **Level: `SpeedDefault`.** `SpeedFastest` costs the same CPU for a much worse
  ratio (9.7x vs 13.6x on a 50k-row table); `SpeedBetterCompression` doubles
  CPU for nothing.
- **Batch: 500 rows** (`defaultBatchSize`). Larger batches compress slightly
  better but delay the first paint: at 2000 rows the first batch grows from
  11.4 KB to 42.8 KB, pushing time-to-first-row on a slow link from 69 ms to
  ~94 ms.

The per-batch flush that `SetDownstreamFlusher` triggers costs nothing in
ratio — measured, it is neutral to 24% *better* than bulk compression, because
block boundaries end up aligned with record batches.
