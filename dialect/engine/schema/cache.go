package schema

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine/connect"
	"github.com/selectDb/toolkit/cache"
)

// Metadata cache keyed by hash(workspaceID, dsn).
var (
	// 10k entries × ~280KB = ~2.7GB max
	metadataCache = cache.New(cache.Options{MaxEntries: 10_000, TTL: 20 * time.Minute})
)

// Schema dump cache keyed by hash(workspaceID, dsn). Stores zstd-compressed DDL.
var (
	dumpCache = cache.New(cache.Options{MaxEntries: 10_000, TTL: 20 * time.Minute})

	dumpEncoder, _ = zstd.NewWriter(nil)
	dumpDecoder, _ = zstd.NewReader(nil)
)

// GetOrFetch returns cached metadata, fetching on miss. refresh=true
// forces a fresh fetch.
//
// Callers asking for the same database while a fetch runs share it, including
// a refresh, and run under the context of the caller that started it. Other
// databases, and cache hits, never wait.
func GetOrFetch(
	ctx context.Context,
	workspaceID, dsn string,
	db *sql.DB,
	dialect core.SQLDialect,
	dbName string,
	refresh bool,
	maxConcurrency ...int,
) (*core.Metadata, error) {
	key := connect.WorkspaceCacheKey(workspaceID, dsn)

	if refresh {
		metadataCache.Delete(key)
	}

	meta, err := metadataCache.GetOrCreate(key, func() (any, error) {
		return Fetch(ctx, db, dialect, dbName, maxConcurrency...)
	})
	if err != nil {
		return nil, err
	}
	return meta.(*core.Metadata), nil
}

// GetOrGenerateDump returns cached DDL, generating on miss. refresh=true forces
// regeneration. Callers asking for the same database while a dump runs share
// it; other databases, and cache hits, never wait.
func GetOrGenerateDump(
	dialect core.SQLDialect,
	workspaceID, dsn string,
	metadata *core.Metadata,
	refresh bool,
) string {
	key := connect.WorkspaceCacheKey(workspaceID, dsn)

	if refresh {
		dumpCache.Delete(key)
	}

	generate := func() string { return Dump(dialect, dsn, metadata) }

	// The caller that generates keeps the plain text rather than decoding what
	// it just compressed.
	var generated string
	var created bool
	compressed, _ := dumpCache.GetOrCreate(key, func() (any, error) {
		generated, created = generate(), true
		return dumpEncoder.EncodeAll([]byte(generated), nil), nil
	})
	if created {
		return generated
	}

	decompressed, err := dumpDecoder.DecodeAll(compressed.([]byte), nil)
	if err != nil {
		sql := generate()
		dumpCache.Set(key, dumpEncoder.EncodeAll([]byte(sql), nil))
		return sql
	}
	return string(decompressed)
}

// Invalidate drops metadata and dump entries for (workspaceID, dsn).
func Invalidate(workspaceID, dsn string) {
	key := connect.WorkspaceCacheKey(workspaceID, dsn)
	metadataCache.Delete(key)
	dumpCache.Delete(key)
}

// InvalidateWorkspace drops every metadata and dump entry of one
// workspace, which is what a workspace being deleted leaves behind otherwise:
// its schema and its DDL, served from memory for the rest of the TTL.
func InvalidateWorkspace(workspaceID string) {
	prefix := connect.WorkspaceKeyPrefix(workspaceID)
	matches := func(key string) bool { return strings.HasPrefix(key, prefix) }

	metadataCache.DeleteFunc(matches)
	dumpCache.DeleteFunc(matches)
}

// ClearCache drops all metadata and dump entries.
func ClearCache() {
	metadataCache.DeleteFunc(func(string) bool { return true })
	dumpCache.DeleteFunc(func(string) bool { return true })
}
