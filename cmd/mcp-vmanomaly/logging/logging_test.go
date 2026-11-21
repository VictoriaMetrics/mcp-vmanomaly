package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func useLogger(t *testing.T, format, level string) *bytes.Buffer {
	t.Helper()
	original := slog.Default()
	t.Cleanup(func() { slog.SetDefault(original) })
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

func TestMiddlewareLogsRequests(t *testing.T) {
	out := useLogger(t, "json", "info")
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			t.Error("wrapped writer must implement http.Flusher for streaming transports")
		}
		w.Header().Set("Mcp-Session-Id", "mcp-session-1")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("12345"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/message?sessionId=secret-session", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	for _, path := range []string{"/health/liveness", "/metrics"} {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("want one access-log line, got %d:\n%s", len(lines), out.String())
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatalf("invalid log line %q: %v", lines[0], err)
	}
	want := map[string]any{
		"msg":        "HTTP request completed",
		"method":     "POST",
		"path":       "/message",
		"status":     float64(http.StatusAccepted),
		"size":       float64(5),
		"session_id": "mcp-session-1",
	}
	for k, v := range want {
		if record[k] != v {
			t.Errorf("%s = %v, want %v", k, record[k], v)
		}
	}
	if strings.Contains(out.String(), "secret-session") {
		t.Fatalf("query string leaked into the access log: %s", out.String())
	}
}

func TestParseLevel(t *testing.T) {
	for in, want := range map[string]slog.Level{"debug": slog.LevelDebug, "info": slog.LevelInfo, "warn": slog.LevelWarn, "error": slog.LevelError, "": slog.LevelInfo} {
		if got := ParseLevel(in); got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}
