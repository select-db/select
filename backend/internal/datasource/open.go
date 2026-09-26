package datasource

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"backend/internal/authz"
	"backend/internal/datasource/cellar"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/dialects"
	"github.com/selectDb/dialect/engine"
	"github.com/selectDb/dialect/engine/connect"
	"github.com/selectDb/dialect/engine/query"
)

// genericConnErr is returned to clients for any datasource connection
// failure. The detail is logged server-side only: a raw dial error leaks
// internal network topology and turns this endpoint into an SSRF oracle.
const genericConnErr = "could not connect to the datasource"

// ErrNotFound is a datasource the caller's workspace does not have.
var ErrNotFound = errors.New("datasource not found")

// Opened is a datasource ready for one request, with the caller's permissions.
type Opened struct {
	ID, WorkspaceID string
	DS              *ResolvedDatasource
	Conn            query.Conn
	Inst            query.DBInstance
}

// Open resolves the datasource id for the request's caller. Errors go through
// OpenError or, in MCP, are matched on ErrNotFound and cellar.ErrOff.
func Open(r *http.Request, id, workspaceID string) (Opened, error) {
	ds, err := GetOrLoadDatasource(r.Context(), id, workspaceID)
	if err != nil {
		return Opened{}, ErrNotFound
	}
	db, err := connect.GetOrOpen(workspaceID, ds.DBType, ds.DSN, ds.SSH, ds.Pool)
	if err != nil {
		return Opened{}, err
	}
	return Opened{
		ID:          id,
		WorkspaceID: workspaceID,
		DS:          ds,
		Conn:        query.Conn{DB: db, Perms: authz.Perms(r)},
		Inst:        query.DBInstance{ID: id, DBType: ds.DBType},
	}, nil
}

// Metadata is the datasource's schema, cached by DSN.
func (o *Opened) Metadata(ctx context.Context, noCache bool) (*core.Metadata, error) {
	dialect := dialects.Get(o.DS.DBType)
	if dialect == nil {
		return nil, fmt.Errorf("unsupported database type: %s", o.DS.DBType)
	}
	return engine.GetOrFetchMetadata(ctx, o.WorkspaceID, o.DS.DSN, o.Conn.DB, dialect, "", noCache)
}

// Stream runs sql into sink, checked against the caller's permissions.
func (o *Opened) Stream(ctx context.Context, sql string, opts query.Options, sink query.RowSink) {
	conn := o.Conn
	meta, err := o.Metadata(ctx, false)
	if err != nil {
		// Without a schema the permission check would refuse the statement
		// and hide why; the reason goes through the same filter as OpenError.
		_, msg := openFailure(err, "datasource stream", o.WorkspaceID, o.ID)
		sink.OnError(errors.New(msg))
		return
	}
	conn.Meta = meta
	query.Stream(ctx, conn, o.Inst, sql, opts, sink)
}

// OpenError answers a request whose datasource could not be opened or reached.
func OpenError(w http.ResponseWriter, err error, logPrefix, workspaceID, dsID string) {
	status, msg := openFailure(err, logPrefix, workspaceID, dsID)
	http.Error(w, msg, status)
}

// openFailure shows only config errors: a raw dial error maps the internal
// network, and a cellar's names its address. The rest is logged.
func openFailure(err error, logPrefix, workspaceID, dsID string) (int, string) {
	var cfgErr *connect.ConfigError
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound, err.Error()
	case errors.Is(err, cellar.ErrOff):
		return http.StatusNotImplemented, err.Error()
	case errors.Is(err, cellar.ErrUnavailable):
		return http.StatusServiceUnavailable, err.Error()
	case errors.As(err, &cfgErr):
		return http.StatusBadGateway, cfgErr.Msg
	}
	log.Printf("%s: ws=%s id=%s: %v", logPrefix, workspaceID, dsID, err)
	return http.StatusBadGateway, genericConnErr
}
