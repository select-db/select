package transport

import (
	"encoding/json"
	"net/http"

	"github.com/klauspost/compress/zstd"
	"github.com/selectDb/dialect/engine/arrowstream"
)

var zstdEncoder, _ = zstd.NewWriter(nil)

// WriteArrow starts a streamed result, the body FetchStream reads for an
// execute; the caller closes the sink.
func WriteArrow(w http.ResponseWriter) *arrowstream.Sink {
	w.Header().Set("Content-Type", "application/vnd.apache.arrow.stream")
	w.Header().Set("Content-Encoding", "zstd")
	w.WriteHeader(http.StatusOK)

	sink := arrowstream.NewSink(w)
	// Flush after every batch so rows reach the client steadily, not in one
	// clump when the handler returns.
	if flusher, ok := w.(http.Flusher); ok {
		sink.SetDownstreamFlusher(flusher.Flush)
	}
	return sink
}

// WriteZstdJSON answers v as zstd-compressed JSON, the body Fetch reads.
func WriteZstdJSON(w http.ResponseWriter, v any) {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Encoding", "zstd")
	_, _ = w.Write(zstdEncoder.EncodeAll(jsonBytes, nil))
}
