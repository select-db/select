package engine

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/selectDb/dialect/core"
	"github.com/selectDb/toolkit/cache"
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
	key := workspaceCacheKey(workspaceID, dsn)

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
	key := workspaceCacheKey(workspaceID, dsn)

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
	key := workspaceCacheKey(workspaceID, dsn)
	metadataCache.Delete(key)
	dumpCache.Delete(key)
}

// InvalidateWorkspaceMetadata drops every metadata and dump entry of one
// workspace, which is what a workspace being deleted leaves behind otherwise:
// its schema and its DDL, served from memory for the rest of the TTL.
func InvalidateWorkspaceMetadata(workspaceID string) {
	prefix := workspaceKeyPrefix(workspaceID)
	matches := func(key string) bool { return strings.HasPrefix(key, prefix) }

	metadataCacheMu.Lock()
	metadataCache.DeleteFunc(matches)
	metadataCacheMu.Unlock()

	dumpCacheMu.Lock()
	dumpCache.DeleteFunc(matches)
	dumpCacheMu.Unlock()
}

// ClearMetadataCache drops all metadata and dump entries.
func ClearMetadataCache() {
	metadataCache.DeleteFunc(func(string) bool { return true })
	dumpCache.DeleteFunc(func(string) bool { return true })
}
