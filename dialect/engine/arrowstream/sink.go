// Package arrowstream provides Arrow IPC + zstd encoding (Sink) and decoding (Stream)
// for streaming database query results over HTTP.
package arrowstream

import (
	"database/sql"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/klauspost/compress/zstd"
)

const defaultBatchSize = 500

// compressionWindow caps how far back zstd may look for matches. The library
// default is 8 MiB, which is live memory held for the whole life of a stream:
// at 8 MiB an encoder costs ~18 MiB of live heap per in-flight query, against
// ~0.14 MiB for the Arrow builders it is wrapping. 1 MiB keeps 85-95% of the
// ratio on real result sets (12.4x -> 11.1x on a 50k-row table, 51x -> 48x on
// a log table) for a 5x cut in per-stream memory. Below 1 MiB the ratio falls
// off sharply for numeric and log-shaped data.
const compressionWindow = 1 << 20

// encoderPool recycles zstd encoders across queries. Constructing one is far
// more expensive than the compression itself for typical result sets — a fresh
// encoder costs ~19 MB of allocation and several ms of CPU before a single row
// is compressed, which a 500-row grid page has nothing to amortise it against.
//
// Encoders are returned only via Sink.Close, which closes the frame and resets
// the encoder off the response writer first. Encoder.Reset purges the match
// history and window, so no bytes and no back-references can cross from one
// query's stream into another's.
var encoderPool = sync.Pool{
	New: func() any {
		// Concurrency 1 keeps compression on the calling goroutine: a single
		// query can no longer fan its own compression across every core, which
		// is what we want on a shared proxy, and it drops the per-stream worker
		// goroutines and their block buffers.
		e, _ := zstd.NewWriter(nil,
			zstd.WithEncoderConcurrency(1),
			zstd.WithWindowSize(compressionWindow),
		)
		return e
	},
}

// Sink encodes query results as Arrow IPC records compressed with zstd.
// Implements the write side: OnColumns → OnRow* → OnDone/OnError.
type Sink struct {
	w               io.Writer
	zw              *zstd.Encoder
	writer          *ipc.Writer
	builder         *array.RecordBuilder
	schema          *arrow.Schema
	alloc           memory.Allocator
	rowsInBatch     int
	batchSize       int
	flushDownstream func()

	colTypes        []*sql.ColumnType
	columnTypeHints []string                       // test-only alternative to colTypes
	fieldEncoders   []func(b array.Builder, v any) // per-column typed append function
}

// NewSink creates a Sink that writes Arrow IPC + zstd to w. The encoder is
// taken from a shared pool; Close must be called to return it.
func NewSink(w io.Writer) *Sink {
	zw := encoderPool.Get().(*zstd.Encoder)
	zw.Reset(w)
	return &Sink{
		w:         w,
		zw:        zw,
		alloc:     memory.NewGoAllocator(),
		batchSize: defaultBatchSize,
	}
}

// SetDownstreamFlusher registers a callback that the sink invokes after every
// batch boundary (including OnExecuted and the per-row batch flushes). Use it
// to push compressed bytes through buffered transports, typically
// http.Flusher.Flush, so consumers see rows promptly instead of in one big
// clump at handler return.
//
// Without a flusher, behaviour is unchanged: zstd holds bytes until its own
// block boundaries and HTTP buffers for ~4KB at a time.
func (s *Sink) SetDownstreamFlusher(flush func()) {
	s.flushDownstream = flush
}

// SetColumnTypes configures typed Arrow columns based on SQL column metadata.
// Must be called before OnColumns. Columns whose type isn't recognized fall
// back to String.
func (s *Sink) SetColumnTypes(colTypes []*sql.ColumnType) {
	s.colTypes = colTypes
}

// SetColumnTypeHints configures typed Arrow columns from raw database type name
// strings (e.g. "INT8", "FLOAT8", "BOOL", "TEXT"). Useful in tests where
// *sql.ColumnType is not available. Must be called before OnColumns.
func (s *Sink) SetColumnTypeHints(dbTypes []string) {
	s.columnTypeHints = dbTypes
}

// OnColumns initializes the Arrow schema from column names.
func (s *Sink) OnColumns(cols []string) error {
	fields := make([]arrow.Field, len(cols))
	s.fieldEncoders = make([]func(array.Builder, any), len(cols))
	for i, name := range cols {
		at, ap := s.resolveType(i)
		fields[i] = arrow.Field{Name: name, Type: at, Nullable: true}
		s.fieldEncoders[i] = ap
	}
	s.schema = arrow.NewSchema(fields, nil)
	s.writer = ipc.NewWriter(s.zw, ipc.WithSchema(s.schema), ipc.WithAllocator(s.alloc))
	s.builder = array.NewRecordBuilder(s.alloc, s.schema)
	return nil
}

func (s *Sink) OnRow(values []any) error {
	for i, v := range values {
		if v == nil {
			s.builder.Field(i).AppendNull()
		} else {
			s.fieldEncoders[i](s.builder.Field(i), v)
		}
	}
	s.rowsInBatch++
	if s.rowsInBatch >= s.batchSize {
		return s.flushBatch()
	}
	return nil
}

// resolveType returns the Arrow type and append function for column i.
func (s *Sink) resolveType(i int) (arrow.DataType, func(array.Builder, any)) {
	var dbType string
	if s.columnTypeHints != nil && i < len(s.columnTypeHints) {
		dbType = strings.ToUpper(s.columnTypeHints[i])
	} else if s.colTypes != nil && i < len(s.colTypes) {
		dbType = strings.ToUpper(s.colTypes[i].DatabaseTypeName())
	} else {
		return arrow.BinaryTypes.String, appendString
	}
	switch {
	case isIntType(dbType):
		return arrow.PrimitiveTypes.Int64, appendInt64
	case isFloatType(dbType):
		return arrow.PrimitiveTypes.Float64, appendFloat64
	case isBoolType(dbType):
		return arrow.FixedWidthTypes.Boolean, appendBool
	default:
		return arrow.BinaryTypes.String, appendString
	}
}

func isIntType(t string) bool {
	switch t {
	case "INT", "INT2", "INT4", "INT8", "INTEGER", "SMALLINT", "BIGINT", "SERIAL", "BIGSERIAL", "TINYINT", "MEDIUMINT":
		return true
	}
	return false
}

func isFloatType(t string) bool {
	switch t {
	case "FLOAT", "FLOAT4", "FLOAT8", "DOUBLE", "REAL", "NUMERIC", "DECIMAL", "MONEY":
		return true
	}
	return false
}

func isBoolType(t string) bool {
	return t == "BOOL" || t == "BOOLEAN"
}

func appendString(b array.Builder, v any) {
	b.(*array.StringBuilder).Append(toString(v))
}

func appendInt64(b array.Builder, v any) {
	switch val := v.(type) {
	case int64:
		b.(*array.Int64Builder).Append(val)
	case int32:
		b.(*array.Int64Builder).Append(int64(val))
	case int:
		b.(*array.Int64Builder).Append(int64(val))
	default:
		// Driver returned an unexpected type; fall back to string column.
		// This shouldn't happen but avoids a panic.
		b.(*array.Int64Builder).Append(toInt64(v))
	}
}

func appendFloat64(b array.Builder, v any) {
	switch val := v.(type) {
	case float64:
		b.(*array.Float64Builder).Append(val)
	case float32:
		b.(*array.Float64Builder).Append(float64(val))
	case int64:
		b.(*array.Float64Builder).Append(float64(val))
	default:
		b.(*array.Float64Builder).Append(toFloat64(v))
	}
}

func appendBool(b array.Builder, v any) {
	switch val := v.(type) {
	case bool:
		b.(*array.BooleanBuilder).Append(val)
	default:
		b.(*array.BooleanBuilder).Append(fmt.Sprintf("%v", val) == "true")
	}
}

func toInt64(v any) int64 {
	n, _ := strconv.ParseInt(fmt.Sprintf("%v", v), 10, 64)
	return n
}

func toFloat64(v any) float64 {
	f, _ := strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
	return f
}

// OnExecuted writes a stand-alone IPC stream segment whose schema metadata
// carries `executed_ms`. Sent before the main row stream so the consumer can
// surface the SQL execution duration as soon as the engine returns from
// QueryContext, even though rows are still streaming.
//
// The Arrow IPC stream format requires at least one schema field, so a dummy
// `_` placeholder is used; the reader keys on the metadata, not the fields.
func (s *Sink) OnExecuted(durationMs int64) {
	meta := arrow.NewMetadata(
		[]string{"executed_ms"},
		[]string{strconv.FormatInt(durationMs, 10)},
	)
	schema := arrow.NewSchema(
		[]arrow.Field{{Name: "_", Type: arrow.BinaryTypes.String}},
		&meta,
	)
	w := ipc.NewWriter(s.zw, ipc.WithSchema(schema), ipc.WithAllocator(s.alloc))
	_ = w.Close()
	_ = s.flushThrough()
}

func (s *Sink) OnDone(rowCount, affected, durationMs int64) error {
	if s.rowsInBatch > 0 {
		if err := s.flushBatch(); err != nil {
			return err
		}
	}
	_ = s.writer.Close()

	// Write summary as a separate stream with metadata
	meta := arrow.NewMetadata([]string{"done", "rows", "affected", "ms"}, []string{
		"true",
		fmt.Sprintf("%d", rowCount),
		fmt.Sprintf("%d", affected),
		fmt.Sprintf("%d", durationMs),
	})
	schema := arrow.NewSchema(s.schema.Fields(), &meta)
	sumWriter := ipc.NewWriter(s.zw, ipc.WithSchema(schema), ipc.WithAllocator(s.alloc))
	_ = sumWriter.Close()
	return nil
}

func (s *Sink) OnError(err error) {
	if s.writer != nil && s.rowsInBatch > 0 {
		_ = s.flushBatch()
		_ = s.writer.Close()
	} else if s.writer != nil {
		_ = s.writer.Close()
	}

	// Write error as metadata in a terminal stream
	meta := arrow.NewMetadata([]string{"error"}, []string{err.Error()})
	var fields []arrow.Field
	if s.schema != nil {
		fields = s.schema.Fields()
	} else {
		fields = []arrow.Field{{Name: "_", Type: arrow.BinaryTypes.String}}
	}
	schema := arrow.NewSchema(fields, &meta)
	errWriter := ipc.NewWriter(s.zw, ipc.WithSchema(schema), ipc.WithAllocator(s.alloc))
	_ = errWriter.Close()
}

// Close flushes the zstd encoder and returns it to the pool. Must be called
// after OnDone or OnError. Idempotent: a second call is a no-op, so the
// encoder can never be handed to two concurrent streams at once.
func (s *Sink) Close() {
	if s.builder != nil {
		s.builder.Release()
		s.builder = nil
	}
	if s.zw == nil {
		return
	}
	_ = s.zw.Close()
	// Reset off the response writer before pooling, so a parked encoder never
	// holds a finished request's connection alive. Reset also clears any write
	// error and the match history, leaving the encoder clean for the next
	// query even when this stream died mid-flight.
	s.zw.Reset(nil)
	encoderPool.Put(s.zw)
	s.zw = nil
}

func (s *Sink) flushBatch() error {
	rec := s.builder.NewRecordBatch()
	defer rec.Release()
	if err := s.writer.Write(rec); err != nil {
		return err
	}
	s.rowsInBatch = 0
	return s.flushThrough()
}

// flushThrough pushes anything zstd has buffered out through the downstream
// transport. No-op when no flusher was registered (the default for tests and
// non-HTTP callers). Flushing per batch buys steady, low-latency emission
// instead of a big clump at stream end, and costs nothing in compression
// ratio: measured across a range of result shapes it is neutral to 24%
// better than bulk compression, because block boundaries end up aligned with
// record batches.
func (s *Sink) flushThrough() error {
	if s.flushDownstream == nil {
		return nil
	}
	if err := s.zw.Flush(); err != nil {
		return err
	}
	s.flushDownstream()
	return nil
}

func toString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}
