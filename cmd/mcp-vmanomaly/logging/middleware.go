package logging

import (
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"
)

// responseWriter records the status code and body size of a response.
type responseWriter struct {
	http.ResponseWriter
	statusCode  int
	size        int
	wroteHeader bool
}

// WriteHeader records the first status code, which is the one sent to the client.
func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.statusCode = code
		rw.wroteHeader = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.wroteHeader = true
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

// isQuietPath reports whether a path is a health or metrics endpoint, matched
// exactly as the auth middleware does, so lookalikes such as /healthz are logged.
func isQuietPath(requestPath string) bool {
	p := path.Clean(requestPath)
	return p == "/metrics" || p == "/health" || strings.HasPrefix(p, "/health/")
}

// sessionID returns the MCP session of a request: the Mcp-Session-Id request or
// response header (Streamable HTTP) or the sessionId query value (SSE messages).
func sessionID(r *http.Request, w http.ResponseWriter) string {
	if id := r.Header.Get("Mcp-Session-Id"); id != "" {
		return id
	}
	if id := w.Header().Get("Mcp-Session-Id"); id != "" {
		return id
	}
	return r.URL.Query().Get("sessionId")
}

// Middleware logs one line per HTTP request to the default slog logger after it completes. Health and
// metrics endpoints are skipped to keep probes and scrapes out of the log.
// Only the path is logged: query strings and headers can carry session IDs or credentials.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isQuietPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		// Long-lived SSE streams only complete on disconnect, so note their start at debug level.
		slog.Debug("HTTP request started",
			"method", r.Method,
			"path", Bounded(r.URL.Path),
			"remote_addr", r.RemoteAddr,
		)

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		slog.Info("HTTP request completed",
			"session_id", Bounded(sessionID(r, wrapped)),
			"method", Bounded(r.Method),
			"path", Bounded(r.URL.Path),
			"remote_addr", r.RemoteAddr,
			"status", wrapped.statusCode,
			"size", wrapped.size,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}
