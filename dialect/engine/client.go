package engine

import (
	"context"
	"fmt"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/dialects"
	"github.com/selectDb/dialect/engine/query"
	"github.com/selectDb/dialect/engine/results"
)

// Client routes queries local vs proxified, manages cancel + result cache.
type Client struct {
	Transport Transport
	// MetadataConcurrency caps concurrent schema-introspection queries during a
	// local metadata fetch. 0 uses the engine default; the desktop app sets 1 so
	// a multi-schema load reuses a single pooled connection instead of bursting.
	MetadataConcurrency int
}

// query.Stream kicks off sql execution and returns a *results.StreamingResult that fills
// asynchronously. The result is registered in the cache under key so subsequent
// Page() calls can read it. The listener (optional) is notified on start /
// progress / done / error.
//
// query.Stream returns immediately; the caller must not assume columns or rows are
// available without observing listener.OnStart or polling result.Header.
func (client *Client) Stream(
	ctx context.Context,
	key string,
	resultID string,
	conn query.Conn,
	instance query.DBInstance,
	workspaceID, sql string,
	options query.Options,
	listener results.StreamListener,
) *results.StreamingResult {
	result := results.NewStreamingResult(resultID)
	results.Set(key, result)

	cancelCtx, cancelFunc := context.WithCancel(ctx)
	unregisterCancel := query.RegisterCancel(key, cancelFunc)

	sink := results.NewStreamingSink(result, listener)

	go func() {
		defer unregisterCancel()

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

		query.Stream(cancelCtx, conn, instance, sql, options, sink)
	}()

	return result
}

// query.Execute runs sql synchronously and returns a buffered query.Result. Used for
// non-interactive paths (export, schema dump) where the full row set is needed
// up front. Does not write to the streaming cache.
func (client *Client) Execute(
	ctx context.Context,
	conn query.Conn,
	instance query.DBInstance,
	workspaceID, sql string,
	options query.Options,
) *query.Result {
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()

	if instance.Proxified {
		stream, err := client.Transport.OpenStream(
			cancelCtx, workspaceID, instance.ID, instance.DBType, sql, options,
		)
		if err != nil {
			return &query.Result{Errors: []string{err.Error()}}
		}
		result := &query.Result{}
		drainStreamInto(stream, query.ResultSink{Result: result})
		return result
	}

	return query.Execute(cancelCtx, conn, instance, sql, options)
}

// Page retrieves a page from a streaming result. Returns nil + false if no
// streaming result exists under key. Status indicates whether the page is
// complete, partial (more rows arriving), or pending (none yet).
func (client *Client) Page(key string, page, pageSize int) (*results.PageData, results.PageStatus, bool) {
	cached, ok := results.Get(key)
	if !ok {
		return nil, results.PagePending, false
	}
	data, status := cached.Page(page, pageSize)
	return data, status, true
}

// Ping checks DB connectivity. Routes through Transport when proxified.
func (client *Client) Ping(ctx context.Context, conn query.Conn, instance query.DBInstance, workspaceID string, noCache bool) error {
	if !instance.Proxified {
		return conn.DB.PingContext(ctx)
	}
	if client.Transport == nil {
		return fmt.Errorf("instance %s is proxified but no transport is configured", instance.ID)
	}
	return client.Transport.Ping(ctx, workspaceID, instance.ID, noCache)
}

// GetMetadata fetches schema. Routes through Transport when proxified.
func (client *Client) GetMetadata(ctx context.Context, conn query.Conn, instance query.DBInstance, workspaceID, dbName string, noCache bool) (*core.Metadata, error) {
	if !instance.Proxified {
		dialect := dialects.Get(instance.DBType)
		if dialect == nil {
			return nil, fmt.Errorf("unsupported database type: %s", instance.DBType)
		}

		return GetOrFetchMetadata(ctx, workspaceID, instance.ID, conn.DB, dialect, dbName, noCache, client.MetadataConcurrency)
	}

	if client.Transport == nil {
		return nil, fmt.Errorf("instance %s is proxified but no transport is configured", instance.ID)
	}
	meta, err := client.Transport.GetMetadata(ctx, workspaceID, instance.ID, noCache)
	if err != nil {
		return nil, err
	}
	// The remote already enriches, but resolve again so the result does not
	// depend on the remote's version or wire encoding of EnumValues.
	core.EnrichEnumValues(meta)
	return meta, nil
}

// DumpSchema returns DDL. For proxified, runs on remote (pg_dump / metadata fallback).
func (client *Client) DumpSchema(ctx context.Context, instance query.DBInstance, workspaceID, dsn string, metadata *core.Metadata, noCache bool) string {
	if instance.Proxified && client.Transport != nil {
		if sql, err := client.Transport.DumpSchema(ctx, workspaceID, instance.ID); err == nil {
			return sql
		}
	}

	dialect := dialects.Get(instance.DBType)
	if dialect == nil {
		return ""
	}

	return GetOrGenerateDump(dialect, workspaceID, dsn, metadata, noCache)
}

// query.Cancel aborts in-flight query under key. No-op if absent.
func (client *Client) Cancel(key string) {
	query.Cancel(key)
}

// drainStreamInto pulls rows from a remote query.RowStream into a results.StreamingResult
// via the supplied listener. Used by the proxified path.
func drainStreamInto(stream query.RowStream, sink query.RowSink) {
	defer func() { _ = stream.Close() }()

	cols, err := stream.Columns()
	if err != nil {
		sink.OnError(err)
		return
	}
	if err := sink.OnColumns(cols); err != nil {
		sink.OnError(err)
		return
	}

	// Forward the early SQL-execution duration when the wire format carries
	// it. Streams that don't expose it return 0; sinks that don't care
	// implement nothing.
	if e, ok := stream.(interface{ Executed() int64 }); ok {
		if executed := e.Executed(); executed > 0 {
			if n, ok := sink.(query.ExecutedSink); ok {
				n.OnExecuted(executed)
			}
		}
	}

	for {
		values, ok, err := stream.Next()
		if err != nil {
			sink.OnError(err)
			return
		}
		if !ok {
			break
		}
		if err := sink.OnRow(values); err != nil {
			sink.OnError(err)
			return
		}
	}

	rowCount, affected, durationMs, err := stream.Summary()
	if err != nil {
		sink.OnError(err)
		return
	}
	_ = sink.OnDone(rowCount, affected, durationMs)
}
