package logging

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// responseWriter records the status code and body size of a response.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

// Flush implements http.Flusher, which the SSE and streamable HTTP transports require.
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// Middleware logs one line per HTTP request to the default slog logger after it completes. Health and
// metrics endpoints are skipped to keep probes and scrapes out of the log.
// Only the path is logged: query strings and headers can carry session IDs or credentials.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/health") || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		// Long-lived SSE streams only complete on disconnect, so note their start at debug level.
		slog.Debug("HTTP request started",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		sessionID := r.Header.Get("Mcp-Session-Id")
		if sessionID == "" {
			sessionID = wrapped.Header().Get("Mcp-Session-Id")
		}
		slog.Info("HTTP request completed",
			"session_id", sessionID,
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
			"status", wrapped.statusCode,
			"size", wrapped.size,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}
