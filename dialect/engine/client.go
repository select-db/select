package engine

import (
	"context"
	"fmt"

	"github.com/selectDb/dialect/core"
)

// Client routes queries local vs proxified, manages cancel + result cache.
type Client struct {
	Transport Transport
	// MetadataConcurrency caps concurrent schema-introspection queries during a
	// local metadata fetch. 0 uses the engine default; the desktop app sets 1 so
	// a multi-schema load reuses a single pooled connection instead of bursting.
	MetadataConcurrency int
}

// Stream kicks off sql execution and returns a *StreamingResult that fills
// asynchronously. The result is registered in the cache under key so subsequent
// Page() calls can read it. The listener (optional) is notified on start /
// progress / done / error.
//
// Stream returns immediately; the caller must not assume columns or rows are
// available without observing listener.OnStart or polling result.Header.
func (client *Client) Stream(
	ctx context.Context,
	key string,
	resultID string,
	conn Conn,
	instance DBInstance,
	workspaceID, sql string,
	options Options,
	listener StreamListener,
) *StreamingResult {
	result := NewStreamingResult(resultID)
	SetStreamingResult(key, result)

	cancelCtx, cancelFunc := context.WithCancel(ctx)
	RegisterCancel(key, cancelFunc)

	sink := NewStreamingSink(result, listener)

	go func() {
		defer UnregisterCancel(key)

		if instance.Proxified {
			stream, err := client.Transport.OpenStream(
				cancelCtx, workspaceID, instance.ID, instance.DBType, sql, options,
			)
			if err != nil {
				sink.OnError(err)
				return
			}
			drainStreamInto(stream, sink)
			return
		}

		StreamLocal(cancelCtx, conn, instance, sql, options, sink)
	}()

	return result
}

// Execute runs sql synchronously and returns a buffered Result. Used for
// non-interactive paths (export, schema dump) where the full row set is needed
// up front. Does not write to the streaming cache.
func (client *Client) Execute(
	ctx context.Context,
	conn Conn,
	instance DBInstance,
	workspaceID, sql string,
	options Options,
) *Result {
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()

	if instance.Proxified {
		stream, err := client.Transport.OpenStream(
			cancelCtx, workspaceID, instance.ID, instance.DBType, sql, options,
		)
		if err != nil {
			return &Result{Errors: []string{err.Error()}}
		}
		result := &Result{}
		drainStreamFull(stream, result)
		return result
	}

	return ExecuteLocal(cancelCtx, conn, instance, sql, options)
}

// Page retrieves a page from a streaming result. Returns nil + false if no
// streaming result exists under key. Status indicates whether the page is
// complete, partial (more rows arriving), or pending (none yet).
func (client *Client) Page(key string, page, pageSize int) (*PageData, PageStatus, bool) {
	cached, ok := GetStreamingResult(key)
	if !ok {
		return nil, PagePending, false
	}
	data, status := cached.Page(page, pageSize)
	return data, status, true
}

// Ping checks DB connectivity. Routes through Transport when proxified.
func (client *Client) Ping(ctx context.Context, conn Conn, instance DBInstance, workspaceID string, noCache bool) error {
	if !instance.Proxified {
		return conn.DB.PingContext(ctx)
	}
	if client.Transport == nil {
		return fmt.Errorf("instance %s is proxified but no transport is configured", instance.ID)
	}
	return client.Transport.Ping(ctx, workspaceID, instance.ID, noCache)
}

// GetMetadata fetches schema. Routes through Transport when proxified.
func (client *Client) GetMetadata(ctx context.Context, conn Conn, instance DBInstance, workspaceID, dbName string, noCache bool) (*core.Metadata, error) {
	if !instance.Proxified {
		dialect := GetDialect(instance.DBType)
		if dialect == nil {
			return nil, fmt.Errorf("unsupported database type: %s", instance.DBType)
		}

		return GetOrFetchMetadata(ctx, workspaceID, instance.ID, conn.DB, dialect, dbName, noCache, client.MetadataConcurrency)
	}

	if client.Transport == nil {
		return nil, fmt.Errorf("instance %s is proxified but no transport is configured", instance.ID)
	}
	meta, err := client.Transport.GetMetadata(ctx, workspaceID, instance.ID)
	if err != nil {
		return nil, err
	}
	// The remote already enriches, but resolve again so the result does not
	// depend on the remote's version or wire encoding of EnumValues.
	core.EnrichEnumValues(meta)
	return meta, nil
}

// DumpSchema returns DDL. For proxified, runs on remote (pg_dump / metadata fallback).
func (client *Client) DumpSchema(ctx context.Context, instance DBInstance, workspaceID, dsn string, metadata *core.Metadata, noCache bool) string {
	if instance.Proxified && client.Transport != nil {
		if sql, err := client.Transport.DumpSchema(ctx, workspaceID, instance.ID); err == nil {
			return sql
		}
	}

	dialect := GetDialect(instance.DBType)
	if dialect == nil {
		return ""
	}

	return GetOrGenerateDump(dialect, workspaceID, dsn, metadata, noCache)
}

// Cancel aborts in-flight query under key. No-op if absent.
func (client *Client) Cancel(key string) {
	Cancel(key)
}

// drainStreamFull reads all rows into result. Used by Execute (non-streaming callers).
func drainStreamFull(stream RowStream, result *Result) {
	defer func() { _ = stream.Close() }()

	columns, err := stream.Columns()
	if err != nil {
		result.Errors = []string{err.Error()}
		return
	}

	result.Columns = columns

	for {
		values, ok, err := stream.Next()
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			break
		}
		if !ok {
			break
		}
		result.Rows = append(result.Rows, values)
	}

	rowCount, affected, durationMs, err := stream.Summary()
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
	}
	result.RowCount = int(rowCount)
	result.AffectedRows = affected
	result.DurationMs = durationMs
}
