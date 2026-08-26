package engine

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/selectDb/toolkit/cache"
	"github.com/selectDb/dialect/core"
)

// Metadata cache keyed by hash(workspaceID, dsn).
var (
	// 10k entries × ~280KB = ~2.7GB max
	metadataCache   = cache.New(cache.Options{MaxEntries: 10_000, TTL: 20 * time.Minute})
	metadataCacheMu sync.Mutex
)

// Schema dump cache keyed by hash(workspaceID, dsn). Stores zstd-compressed DDL.
var (
	dumpCache   = cache.New(cache.Options{MaxEntries: 10_000, TTL: 20 * time.Minute})
	dumpCacheMu sync.Mutex

	dumpEncoder, _ = zstd.NewWriter(nil)
	dumpDecoder, _ = zstd.NewReader(nil)
)

// GetOrFetchMetadata returns cached metadata, fetching on miss.
// refresh=true forces fresh fetch. Serialised by mu.
func GetOrFetchMetadata(
	ctx context.Context,
	workspaceID, dsn string,
	db *sql.DB,
	dialect core.SQLDialect,
	dbName string,
	refresh bool,
	maxConcurrency ...int,
) (*core.Metadata, error) {
	key := hashWorkspaceDSN(workspaceID, dsn)

	metadataCacheMu.Lock()
	defer metadataCacheMu.Unlock()

	if !refresh {
		if v, ok := metadataCache.Get(key); ok {
			return v.(*core.Metadata), nil
		}
	}

	meta, err := FetchMetadata(ctx, db, dialect, dbName, maxConcurrency...)
	if err != nil {
		return nil, err
	}
	metadataCache.Set(key, meta)
	return meta, nil
}

// GetOrGenerateDump returns cached DDL, generating on miss.
// refresh=true forces regeneration.
func GetOrGenerateDump(
	dialect core.SQLDialect,
	workspaceID, dsn string,
	metadata *core.Metadata,
	refresh bool,
) string {
	key := hashWorkspaceDSN(workspaceID, dsn)

	dumpCacheMu.Lock()
	defer dumpCacheMu.Unlock()

	if !refresh {
		if v, ok := dumpCache.Get(key); ok {
			decompressed, err := dumpDecoder.DecodeAll(v.([]byte), nil)
			if err == nil {
				return string(decompressed)
			}
		}
	}

	sql := DumpSchema(dialect, dsn, metadata)
	dumpCache.Set(key, dumpEncoder.EncodeAll([]byte(sql), nil))
	return sql
}

// InvalidateMetadata drops metadata and dump entries for (workspaceID, dsn).
func InvalidateMetadata(workspaceID, dsn string) {
	key := hashWorkspaceDSN(workspaceID, dsn)
	metadataCache.Delete(key)
	dumpCache.Delete(key)
}

// ClearMetadataCache drops all metadata and dump entries.
func ClearMetadataCache() {
	metadataCache.DeleteFunc(func(string) bool { return true })
	dumpCache.DeleteFunc(func(string) bool { return true })
}
