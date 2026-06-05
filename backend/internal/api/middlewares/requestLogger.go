package middlewares

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"backend/internal/utils"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
	body   *bytes.Buffer
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

var (
	green = "\033[32m"
	red   = "\033[31m"
	cyan  = "\033[36m"
	gray  = "\033[90m"
)

func RequestLogger(next http.Handler) http.Handler {
	if os.Getenv("LOG_HTTP") != "true" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := utils.GenerateRequestID()

		var bodyCopy []byte
		if r.Body != nil {
			bodyCopy, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewBuffer(bodyCopy))
		}

		log.Printf("%s %-8s [%s] ──▶ %-6s %-30s",
			utils.Colorize("[CLIENT] ▶▶▶ HTTP", cyan), timestamp(), requestID, r.Method, r.URL.Path,
		)
		if len(bodyCopy) > 0 {
			utils.LogIndentedJSONOrRaw(bodyCopy)
		}

		rec := &responseRecorder{ResponseWriter: w, body: &bytes.Buffer{}, status: 200}
		start := time.Now()
		next.ServeHTTP(rec, r)
		duration := time.Since(start)

		statusColor := green
		if rec.status >= 400 {
			statusColor = red
		}

		log.Printf("%s %-8s [%s] ◀── %-6s %-30s [%s%d%s] (%s)",
			utils.Colorize("[CLIENT] ◀◀◀ HTTP", cyan), timestamp(), requestID, r.Method, r.URL.Path,
			statusColor, rec.status, gray, duration.Round(time.Millisecond),
		)

		if rec.body.Len() > 0 {
			utils.LogIndentedJSONOrRaw(rec.body.Bytes())
		}
	})
}

func timestamp() string {
	return time.Now().Format("15:04:05")
}
