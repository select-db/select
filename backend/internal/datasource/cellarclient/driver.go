package cellarclient

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"backend/internal/cellar"

	"github.com/selectDb/dialect/engine/arrowstream"
	"github.com/selectDb/dialect/sqlite"
)

// The SQLite dialect opens a managed database's DSN with this driver, which
// sends each statement to the cellar instead of opening a file.
func init() {
	sql.Register(sqlite.CellarDriver, sqlDriver{})
}

// ErrUnavailable is a cellar the backend could not reach. The cause, which
// names the cellar's address, is only logged.
var ErrUnavailable error = &arrowstream.Error{Code: cellar.CodeUnavailable, Message: "managed database temporarily unavailable, retry"}

var (
	errNoPrepare = errors.New("cellar: prepared statements are not supported")
	errNoTx      = errors.New("cellar: transactions are not supported")
)

type sqlDriver struct{}

// QueryRunsAll: the cellar runs any statement as a query, so a failed one is
// never resent as an exec.
func (sqlDriver) QueryRunsAll() {}

func (sqlDriver) Open(dsn string) (driver.Conn, error) {
	id, grant, err := parseDSN(dsn)
	if err != nil {
		return nil, err
	}
	return conn{path: "/datasources/" + id + "/query", grant: grant}, nil
}

// parseDSN reads a managed database's id, and the grant every request to its
// cellar carries, from the DSN the backend built for it.
func parseDSN(dsn string) (id string, grant string, err error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", "", err
	}
	params := parsed.Query()
	maxBytes, _ := strconv.ParseInt(params.Get("max_bytes"), 10, 64)
	maxInFlight, _ := strconv.Atoi(params.Get("max_in_flight"))
	grant, err = cellar.Grant{
		WorkspaceID: params.Get("workspace_id"),
		CellarID:    parsed.Host,
		MaxBytes:    maxBytes,
		MaxInFlight: maxInFlight,
	}.Encode()
	return strings.TrimPrefix(parsed.Path, "/"), grant, err
}

// conn sends each statement to the cellar; it holds no state between them.
type conn struct {
	path, grant string
}

func (conn) Prepare(string) (driver.Stmt, error) { return nil, errNoPrepare }
func (conn) Begin() (driver.Tx, error)           { return nil, errNoTx }
func (conn) Close() error                        { return nil }

// CheckNamedValue keeps arguments to strings, which JSON carries unchanged.
func (conn) CheckNamedValue(nv *driver.NamedValue) error {
	switch nv.Value.(type) {
	case nil, string:
		return nil
	}
	return fmt.Errorf("cellar: argument %d is a %T; only strings are supported", nv.Ordinal, nv.Value)
}

func (c conn) Ping(ctx context.Context) error {
	_, err := c.ExecContext(ctx, "SELECT 1", nil)
	return err
}

func (c conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	body, err := c.send(ctx, query, args)
	if err != nil {
		return nil, err
	}
	stream, err := arrowstream.NewStream(body)
	if err != nil {
		_ = body.Close()
		return nil, err
	}
	cols, err := stream.Columns()
	if err != nil {
		_ = stream.Close()
		return nil, err
	}
	return &rows{stream: stream, cols: cols}, nil
}

func (c conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	r, err := c.QueryContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	stream := r.(*rows).stream
	defer func() { _ = stream.Close() }()
	for {
		if _, ok, err := stream.Next(); err != nil {
			return nil, err
		} else if !ok {
			break
		}
	}
	_, affected, _, err := stream.Summary()
	return driver.RowsAffected(affected), err
}

func (c conn) send(ctx context.Context, query string, args []driver.NamedValue) (io.ReadCloser, error) {
	values := make([]any, len(args))
	for i, arg := range args {
		values[i] = arg.Value
	}
	body, err := json.Marshal(cellar.Query{SQL: query, Args: values})
	if err != nil {
		return nil, err
	}
	resp, err := request(ctx, http.MethodPost, c.path, c.grant, body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		// Failures inside a query arrive in the stream, already classified;
		// only the cellar's middlewares answer with a status.
		defer func() { _ = resp.Body.Close() }()
		// InFlight answers 408 when no slot freed up within the statement's time.
		if resp.StatusCode == http.StatusRequestTimeout {
			return nil, &arrowstream.Error{Code: cellar.CodeTimeout, Message: "managed database busy: no free slot within the time limit, retry"}
		}
		errorBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, cellar.InternalError(fmt.Sprintf("cellar: %s: %d %s", c.path, resp.StatusCode, strings.TrimSpace(string(errorBody))))
	}
	return resp.Body, nil
}

// rows reads the cellar's Arrow stream, whose values arrive as strings;
// database/sql converts them on Scan.
type rows struct {
	stream *arrowstream.Stream
	cols   []string
}

func (r *rows) Columns() []string { return r.cols }
func (r *rows) Close() error      { return r.stream.Close() }

func (r *rows) Next(dest []driver.Value) error {
	values, ok, err := r.stream.Next()
	if err != nil {
		return err
	}
	if !ok {
		return io.EOF
	}
	for i, v := range values {
		dest[i] = v
	}
	return nil
}
