package cellar

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"backend/internal/auth"
	server "backend/internal/cellar"

	"github.com/selectDb/dialect/engine/arrowstream"
	"github.com/selectDb/dialect/sqlite"
	"github.com/selectDb/toolkit/cache"
)

// Scheme is the scheme of a managed database's DSN, and the name of the
// database/sql driver that opens it: engine.GetOrOpenConn opens it like any
// other datasource.
const Scheme = "cellar"

func init() {
	sql.Register(Scheme, sqlDriver{})
	sqlite.CellarDriver = Scheme
}

// ErrUnavailable is a cellar the backend could not reach. The cause, which
// names the cellar's address, is only logged.
var ErrUnavailable = errors.New("managed database temporarily unavailable, retry")

var (
	errNoPrepare = errors.New("cellar: prepared statements are not supported")
	errNoTx      = errors.New("cellar: transactions are not supported")

	httpClient = &http.Client{}
)

// The service token lives tokenTTL and is reused for the 50s window it was
// signed in, so it has 10s left when it reaches the cellar. Each sign is a KMS call.
const (
	tokenTTL = 60 * time.Second
	reuseFor = 50 * time.Second
)

var tokens = cache.New(cache.Options{MaxEntries: 1})

type sqlDriver struct{}

// QueryRunsAll: the cellar runs any statement as a query, so a failed one is
// never resent as an exec.
func (sqlDriver) QueryRunsAll() {}

func (d sqlDriver) Open(dsn string) (driver.Conn, error) {
	c, err := d.OpenConnector(dsn)
	if err != nil {
		return nil, err
	}
	return c.Connect(context.Background())
}

func (sqlDriver) OpenConnector(dsn string) (driver.Connector, error) {
	if URL == "" {
		return nil, ErrOff
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	maxBytes, _ := strconv.ParseInt(q.Get("max_bytes"), 10, 64)
	maxInFlight, _ := strconv.Atoi(q.Get("max_in_flight"))
	grant, err := server.Grant{
		WorkspaceID: q.Get("workspace_id"),
		CellarID:    u.Host,
		MaxBytes:    maxBytes,
		MaxInFlight: maxInFlight,
	}.Encode()
	if err != nil {
		return nil, err
	}
	return conn{path: "/datasources" + u.Path + "/query", grant: grant}, nil
}

// conn sends each statement to the cellar; it holds no state between them.
type conn struct {
	path, grant string
}

func (c conn) Connect(context.Context) (driver.Conn, error) { return c, nil }
func (conn) Driver() driver.Driver                          { return sqlDriver{} }
func (conn) Prepare(string) (driver.Stmt, error)            { return nil, errNoPrepare }
func (conn) Begin() (driver.Tx, error)                      { return nil, errNoTx }
func (conn) Close() error                                   { return nil }

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
	if URL == "" {
		return nil, ErrOff
	}
	values := make([]any, len(args))
	for i, a := range args {
		values[i] = a.Value
	}
	body, err := json.Marshal(server.Query{SQL: query, Args: values})
	if err != nil {
		return nil, err
	}
	window := strconv.FormatInt(time.Now().Unix()/int64(reuseFor.Seconds()), 10)
	token, err := tokens.GetOrCreate(window, func() (any, error) {
		return auth.Sign(auth.CustomClaims{}, server.Audience, tokenTTL)
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, URL+c.path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.(string))
	req.Header.Set(server.GrantHeader, c.grant)
	resp, err := httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		log.Printf("cellar: %s: %v", c.path, err)
		return nil, ErrUnavailable
	}
	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("cellar %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
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
