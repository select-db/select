package datasource

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"backend/internal/authz"
	"backend/internal/cellar"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/dialects"
	"github.com/selectDb/dialect/engine/arrowstream"
	"github.com/selectDb/dialect/engine/connect"
	"github.com/selectDb/dialect/engine/query"
	"github.com/selectDb/dialect/engine/schema"
)

// genericConnErr is returned to clients for any datasource connection
// failure. The detail is logged server-side only: a raw dial error leaks
// internal network topology and turns this endpoint into an SSRF oracle.
const genericConnErr = "could not connect to the datasource"

// ErrNotFound is a datasource the caller's workspace does not have.
var ErrNotFound = errors.New("datasource not found")

// Refusal is a request the caller can correct, answered with its HTTP status.
type Refusal struct {
	Status  int
	Message string
}

func (refusal *Refusal) Error() string { return refusal.Message }

var errForbidden = &Refusal{http.StatusForbidden, "forbidden"}

// Opened is a datasource ready for one request, with the caller's permissions.
type Opened struct {
	ID, WorkspaceID string
	DS              *ResolvedDatasource
	Conn            query.Conn
	Inst            query.Datasource
}

// Open resolves the datasource id for the request's caller. Errors go through
// OpenError or, in MCP, asToolError.
func Open(r *http.Request, id, workspaceID string) (Opened, error) {
	ds, err := GetOrLoadDatasource(r.Context(), id, workspaceID)
	var coded *arrowstream.Error
	if errors.As(err, &coded) {
		return Opened{}, err
	}
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
		Inst:        query.Datasource{ID: id, DBType: ds.DBType},
	}, nil
}

// Metadata is the datasource's schema, cached by DSN.
func (o *Opened) Metadata(ctx context.Context, noCache bool) (*core.Metadata, error) {
	dialect := dialects.Get(o.DS.DBType)
	if dialect == nil {
		return nil, fmt.Errorf("unsupported database type: %s", o.DS.DBType)
	}
	return schema.GetOrFetch(ctx, o.WorkspaceID, o.DS.DSN, o.Conn.DB, dialect, "", noCache)
}

// Stream runs sql into sink, checked against the caller's permissions.
func (o *Opened) Stream(ctx context.Context, sql string, opts query.Options, sink query.RowSink) {
	conn := o.Conn
	meta, err := o.Metadata(ctx, false)
	if err != nil {
		// Without a schema the permission check would refuse the statement
		// and hide why; the reason goes through the same filter as OpenError.
		_, shown := openFailure(err, "datasource stream", o.WorkspaceID, o.ID)
		sink.OnError(shown)
		return
	}
	conn.Meta = meta
	query.Stream(ctx, conn, o.Inst, sql, opts, sink)
}

// OpenError answers a request whose datasource could not be opened or reached.
func OpenError(w http.ResponseWriter, err error, logPrefix, workspaceID, datasourceID string) {
	status, shown := openFailure(err, logPrefix, workspaceID, datasourceID)
	if status == http.StatusServiceUnavailable {
		w.Header().Set("Retry-After", "5")
	}
	http.Error(w, shown.Error(), status)
}

// codeStatus is the HTTP status of each code a managed database's failure carries.
var codeStatus = map[string]int{
	cellar.CodeSQLError:           http.StatusBadRequest,
	cellar.CodeForbiddenStatement: http.StatusBadRequest,
	cellar.CodeQuotaExceeded:      http.StatusForbidden,
	cellar.CodeTimeout:            http.StatusRequestTimeout,
	cellar.CodeWaking:             http.StatusServiceUnavailable,
	cellar.CodeUnavailable:        http.StatusServiceUnavailable,
	cellar.CodeDisabled:           http.StatusNotImplemented,
	cellar.CodeInternal:           http.StatusInternalServerError,
}

// openFailure returns the status and the error a caller may see. A managed
// database's failure is already classified; of the rest, only config errors
// are shown, since a raw dial error maps the internal network. The rest is logged.
func openFailure(err error, logPrefix, workspaceID, datasourceID string) (int, error) {
	var coded *arrowstream.Error
	var cfgErr *connect.ConfigError
	var refused *Refusal
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound, err
	case errors.As(err, &refused):
		return refused.Status, refused
	case errors.As(err, &coded) && codeStatus[coded.Code] != 0:
		return codeStatus[coded.Code], coded
	case errors.As(err, &cfgErr):
		return http.StatusBadGateway, cfgErr
	}
	log.Printf("%s: ws=%s id=%s: %v", logPrefix, workspaceID, datasourceID, err)
	return http.StatusBadGateway, errors.New(genericConnErr)
}
