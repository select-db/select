# Spike: Litestream v0.5 as a library, and what a cellar can carry

Litestream v0.5.17 (pinned), `modernc.org/sqlite`, file replica. Measured on
2 cores of a Xeon 2.1 GHz (`taskset -c 0,1`, `GOMAXPROCS=2`), local SSD, to
approach a d2-4. Network to the bucket is not measured here: see "Not measured".

## What the library gives

| Need in the plan | Litestream v0.5 | Works |
| --- | --- | --- |
| One replica per db at `dbs/{db_id}/` | `NewDB` + `NewReplicaWithClient` per db | yes |
| Add and drop dbs while running | `Store.RegisterDB` / `Store.UnregisterDB` | yes |
| Replica holds everything (before evict) | `DB.SyncStatus`: local vs remote TXID, `InSync` | yes |
| Restore latest (wake) | `Replica.Restore` with `OutputPath` | yes |
| Restore at a time (fork `at`) | `RestoreOptions.Timestamp` | yes, exact rows checked |
| Directory replica (dev, on-prem) | `file.NewReplicaClient` | yes |
| Retention per plan (1 or 7 days) | store-wide only: `SnapshotInterval`, `SnapshotRetention` | no |

Constraints the code must respect:

- Litestream opens the db with the same driver; `modernc.org/sqlite` only.
- A registered db cannot be unregistered while our pool holds connections to
  it: close the pool (`connect.DeleteConnsByAddr`) first.
- It keeps working files next to the db in `.<id>.db-litestream/`. Delete and
  the inventory must treat that directory as part of the db.

## Numbers

### Per db kept replicating (registered)

| | per db | 1000 dbs |
| --- | --- | --- |
| RAM | 350 KB | 350 MB |
| goroutines | 4 | 4000 |
| open files | 6 | 6000 |
| CPU when idle, check every 1s | 0.036% of a core | 36% of a core |
| CPU when idle, check every 10s | 0.025% of a core | 25% of a core |
| register | 3.5 ms | 3.5 s |

The idle cost comes from Litestream's per-db loops, not the check interval.

### Per open db (our pool, after a full scan)

About 4 MB of RAM, almost all SQLite page cache. Closing idle connections is
what keeps this bounded.

### Queries through the cellar (HTTP handler in process, 100 MB db, no replication)

| | 1 client | 16 clients |
| --- | --- | --- |
| point read by id | 0.23 ms, 4,300/s | 10,500/s |
| 1000-row range | 2.8 ms, 360/s | 725/s |
| aggregate over 800k rows (100 MB) | 110 ms | 14/s |
| single-row insert | 0.57 ms, 1,750/s | 1,675/s (one writer per db) |

A point read spends about 22% verifying the service token (RSA, on every
request) and 18% compressing the Arrow stream (zstd). Both are cheap to cut
later if reads become the bottleneck.

### Replication cost on writes (raw SQLite, one writer)

| | tx/s | p50 | p99 | CPU |
| --- | --- | --- | --- | --- |
| no replication | 41,000 | 10 us | 55 us | 64% of a core |
| replication | 11,000 | 71 us | 280 us | 98% of a core |

The cellar's own write path tops out near 1,750/s per db, so replication is
not what limits writes. The replica caught up 11 ms after the load stopped.

### Copy, restore, evict

| size | fork or download (`VACUUM INTO`) | first full sync | restore latest | restore at a time | evict |
| --- | --- | --- | --- | --- | --- |
| 10 MB | 0.07 s | 0.1 s | 0.2 s | 0.3 s | 0.2 s |
| 100 MB | 0.5 s | 1.5 s | 2.6 s | 2.8 s | 2.6 s |
| 250 MB | 1.7 s | 1.7 s | 4.6 s | 5.9 s | 5.1 s |
| 1 GB | 8.3 s | 6.8 s | 21 s | 14 s | not run |

Restore reads at 50 to 70 MB/s from local disk, before any network. Evict is
checkpoint, sync, check `InSync`, unregister, delete.

### Bucket storage

A snapshot of wordy business data is 0.44x the db. The replica keeps one
snapshot per `SnapshotInterval` inside `SnapshotRetention`, plus the changes
between them, which are small for append work (0.2 MB for 12,000 rows).

| snapshot every | kept | snapshots held | bucket per db |
| --- | --- | --- | --- |
| 1 day | 7 days | 8 | about 3.5x the db |
| 7 days | 7 days | 1 to 2 | about 0.5x to 1x the db, plus 7 days of changes |

Worst case per plan with daily snapshots: Solo 1 GB of dbs, 3.5 GB of bucket;
Teams 20 GB, 70 GB of bucket. A db that rewrites most of its pages often
(bulk updates) adds close to its size in changes per compaction window.

### Bucket requests

Derived from the intervals, not counted: a db written continuously uploads
one change file per second, plus compactions (every 30s, 5m, 1h) and a daily
snapshot, about 3,700 writes an hour. An idle db uploads nothing.

## What a d2-4 carries (2 vCPU, 4 GB RAM, 50 GB disk)

- Dbs kept replicating: a few hundred. 300 idle cost about 11% of a core,
  105 MB of RAM and 1,800 open files (raise `LimitNOFILE` to 8192).
- Dbs open at once: RAM allows about 500 at full page cache, next to the
  replicating set.
- Disk holds only the hot set; the bucket holds the rest.
- Reads: about 10,000 point reads/s or 700 1000-row scans/s across all dbs,
  before the network between backend and cellar.
- Writes: about 1,700 single-row inserts/s per db; separate dbs write in
  parallel, bounded by the 2 cores.
- Wake: about 0.25 s for a 10 MB db and 4.6 s for 250 MB from local disk, plus
  the download. A 1 GB db exceeds the planned 15 s wait even before the
  network.

## Decisions this leads to

1. **One `Store`, 7 days kept for every plan.** Retention cannot be per db, and
   the backend already refuses an `at` outside the plan's window. Solo keeps
   history it cannot use; that costs storage, not code.
2. **Replicate only what is in use.** A db idle for 15 minutes and `InSync` is
   evicted: pool closed, unregistered, local files deleted. Disk pressure uses
   the same routine, least recently used first. One mechanism, no warm tier.
3. **Snapshot interval is the storage lever.** Daily gives a fast restore and
   3.5x storage; weekly gives 1x and replays up to 7 days of changes on
   restore. Start daily, measure bucket cost, move to weekly if it matters.
4. **Keep the 15 s wake wait**, and answer `waking` above it. It fits Solo's
   250 MB cap from local disk, but not a 1 GB Teams db.

## Not measured

- The OVH bucket: `NoncurrentVersionExpiration`, upload and download speed
  from a d2-4, request latency. Wake time is restore time plus the download,
  so this decides whether Teams dbs wake inside 15 s.
- Restore speed when a snapshot is 7 days old and many change files replay.
- CPU and memory under many dbs written at once.
