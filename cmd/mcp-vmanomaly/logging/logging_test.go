package logging

import (
	"bytes"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func useLogger(t *testing.T, format, level string) *bytes.Buffer {
	t.Helper()
	original, originalOutput, originalFlags := slog.Default(), log.Writer(), log.Flags()
	t.Cleanup(func() {
		slog.SetDefault(original)
		log.SetOutput(originalOutput)
		log.SetFlags(originalFlags)
	})
	var out bytes.Buffer
	if _, err := New(&out, format, level); err != nil {
		t.Fatalf("New failed: %v", err)
	}
	return &out
}

func TestNewFormats(t *testing.T) {
	out := useLogger(t, "json", "info")
	slog.Info("hello", "key", "value")
	slog.Debug("hidden")
	var record map[string]any
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatalf("json format did not write one JSON record: %v: %q", err, out.String())
	}
	if record["msg"] != "hello" || record["key"] != "value" {
		t.Fatalf("unexpected record %v", record)
	}

	out = useLogger(t, "text", "debug")
	slog.Debug("visible")
	if !strings.Contains(out.String(), "msg=visible") {
		t.Fatalf("text format at debug level did not log: %q", out.String())
	}

	if _, err := New(&bytes.Buffer{}, "xml", "info"); err == nil {
		t.Fatal("expected an error for an unknown format")
	}
}

func TestStandardLogPackageLogsAtErrorLevel(t *testing.T) {
	out := useLogger(t, "json", "error")
	log.Printf("mcp-go: panic in hook")
	var record map[string]any
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatalf("standard log output was not routed to slog at error level: %q", out.String())
	}
	if record["level"] != "ERROR" || record["msg"] != "mcp-go: panic in hook" {
		t.Fatalf("unexpected record %v", record)
	}
}

func TestBounded(t *testing.T) {
	if got := Bounded("short"); got != "short" {
		t.Fatalf("Bounded(short) = %q", got)
	}
	got := Bounded(strings.Repeat("x", 1000))
	if len(got) > maxFieldLength+len("...(truncated)") || !strings.HasSuffix(got, "...(truncated)") {
		t.Fatalf("Bounded did not truncate: %d bytes", len(got))
	}
}

func decodeLines(t *testing.T, out *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("invalid log line %q: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

func TestMiddlewareLogsRequests(t *testing.T) {
	out := useLogger(t, "json", "info")
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mcp" {
			w.Header().Set("Mcp-Session-Id", "mcp-session-1")
		}
		w.WriteHeader(http.StatusAccepted)
		w.WriteHeader(http.StatusInternalServerError) // ignored by net/http; must not be logged
		_, _ = w.Write([]byte("12345"))
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/mcp", nil))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/message?sessionId=sse-1&token=do-not-log", nil))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))
	for _, path := range []string{"/health/liveness", "/health/readiness", "/metrics"} {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	records := decodeLines(t, out)
	if len(records) != 3 {
		t.Fatalf("want 3 access-log lines (/mcp, /message, /healthz), got %d:\n%s", len(records), out.String())
	}
	for i, want := range []map[string]any{
		{"path": "/mcp", "session_id": "mcp-session-1"},
		{"path": "/message", "session_id": "sse-1"},
		{"path": "/healthz", "session_id": ""},
	} {
		for k, v := range want {
			if records[i][k] != v {
				t.Errorf("line %d: %s = %v, want %v", i, k, records[i][k], v)
			}
		}
		if records[i]["msg"] != "HTTP request completed" || records[i]["status"] != float64(http.StatusAccepted) || records[i]["size"] != float64(5) {
			t.Errorf("line %d: unexpected record %v", i, records[i])
		}
	}
	if strings.Contains(out.String(), "do-not-log") {
		t.Fatalf("query string leaked into the access log: %s", out.String())
	}
}

func TestMiddlewareBoundsClientValues(t *testing.T) {
	out := useLogger(t, "json", "info")
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Mcp-Session-Id", strings.Repeat("s", 10_000))
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if out.Len() > 2_000 {
		t.Fatalf("access-log line is not bounded: %d bytes", out.Len())
	}
}

func TestMiddlewareForwardsFlush(t *testing.T) {
	useLogger(t, "json", "info")
	rec := httptest.NewRecorder()
	Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("wrapped writer must implement http.Flusher for streaming transports")
		}
		_, _ = w.Write([]byte("event: message\n\n"))
		flusher.Flush()
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sse", nil))
	if !rec.Flushed {
		t.Fatal("Flush was not forwarded to the underlying writer")
	}
}

func TestParseLevel(t *testing.T) {
	for in, want := range map[string]slog.Level{"debug": slog.LevelDebug, "info": slog.LevelInfo, "warn": slog.LevelWarn, "error": slog.LevelError, "": slog.LevelInfo} {
		if got := ParseLevel(in); got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}
