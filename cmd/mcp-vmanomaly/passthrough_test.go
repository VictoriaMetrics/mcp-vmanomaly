package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/mcp-vmanomaly/cmd/mcp-vmanomaly/config"
)

const healthCheckCall = `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"vmanomaly_health_check","arguments":{}}}`

// fakeVmanomaly records the headers of the last request it received.
type fakeVmanomaly struct {
	mu      sync.Mutex
	headers http.Header
}

func (f *fakeVmanomaly) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.headers = r.Header.Clone()
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (f *fakeVmanomaly) lastHeaders() http.Header {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.headers
}

func newTestHTTPHandler(t *testing.T, env map[string]string) (http.Handler, *fakeVmanomaly) {
	t.Helper()
	backend := &fakeVmanomaly{}
	upstream := httptest.NewServer(backend)
	t.Cleanup(upstream.Close)

	defaults := map[string]string{
		"VMANOMALY_ENDPOINT":          upstream.URL,
		"VMANOMALY_BEARER_TOKEN":      "",
		"VMANOMALY_BEARER_TOKEN_FILE": "",
		"VMANOMALY_HEADERS":           "",
		"MCP_SERVER_MODE":             "http",
		"MCP_ENABLED_TOOLS":           "",
		"MCP_DISABLED_TOOLS":          "",
		"MCP_DISABLE_RESOURCES":       "true",
		"MCP_AUTH_TOKEN":              "",
		"MCP_AUTH_TOKEN_FILE":         "",
		"MCP_PASSTHROUGH_HEADERS":     "",
	}
	for k, v := range env {
		defaults[k] = v
	}
	for k, v := range defaults {
		t.Setenv(k, v)
	}
	cfg, err := config.InitConfig()
	if err != nil {
		t.Fatalf("InitConfig failed: %v", err)
	}
	ms := metrics.NewSet()
	mcpServer, err := newMCPServer(cfg, ms, false)
	if err != nil {
		t.Fatalf("newMCPServer failed: %v", err)
	}
	var ready atomic.Bool
	ready.Store(true)
	handler, err := newHTTPHandler(cfg, mcpServer, ms, &ready)
	if err != nil {
		t.Fatalf("newHTTPHandler failed: %v", err)
	}
	return handler, backend
}

func postMCP(handler http.Handler, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// callHealthCheck opens a session like an MCP client and calls vmanomaly_health_check,
// sending headers with both requests.
func callHealthCheck(t *testing.T, handler http.Handler, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	init := postMCP(handler, initializeRequest, headers)
	if init.Code != http.StatusOK {
		return init
	}
	sessionID := init.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("initialize returned no Mcp-Session-Id")
	}
	withSession := map[string]string{"Mcp-Session-Id": sessionID}
	for k, v := range headers {
		withSession[k] = v
	}
	return postMCP(handler, healthCheckCall, withSession)
}

func TestPassthroughHeadersReachVmanomaly(t *testing.T) {
	handler, backend := newTestHTTPHandler(t, map[string]string{
		"VMANOMALY_BEARER_TOKEN":  "static-token",
		"VMANOMALY_HEADERS":       "X-Tenant=static,X-Static=kept",
		"MCP_PASSTHROUGH_HEADERS": "x-tenant,Authorization",
	})

	rec := callHealthCheck(t, handler, map[string]string{
		"Authorization":  "Bearer user-token",
		"X-Tenant":       "team-a",
		"X-Not-Selected": "dropped",
	})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `\"status\": \"ok\"`) {
		t.Fatalf("health check failed: %d %s", rec.Code, rec.Body.String())
	}

	got := backend.lastHeaders()
	for name, want := range map[string]string{
		"Authorization":  "Bearer user-token",
		"X-Tenant":       "team-a",
		"X-Static":       "kept",
		"X-Not-Selected": "",
	} {
		if got.Get(name) != want {
			t.Errorf("vmanomaly received %s=%q, want %q", name, got.Get(name), want)
		}
	}
}

func TestStaticHeadersWithoutPassthrough(t *testing.T) {
	handler, backend := newTestHTTPHandler(t, map[string]string{
		"VMANOMALY_BEARER_TOKEN": "static-token",
		"VMANOMALY_HEADERS":      "X-Tenant=static",
	})

	rec := callHealthCheck(t, handler, map[string]string{
		"Authorization": "Bearer user-token",
		"X-Tenant":      "team-a",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("health check failed: %d %s", rec.Code, rec.Body.String())
	}
	got := backend.lastHeaders()
	if got.Get("Authorization") != "Bearer static-token" || got.Get("X-Tenant") != "static" {
		t.Fatalf("client headers must not be forwarded unless listed: %v", got)
	}
}

func TestAuthTokenProtectsMCPEndpoint(t *testing.T) {
	handler, backend := newTestHTTPHandler(t, map[string]string{
		"MCP_AUTH_TOKEN":          "mcp-secret",
		"MCP_PASSTHROUGH_HEADERS": "X-Tenant",
	})

	if rec := postMCP(handler, initializeRequest, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated request status = %d, want 401", rec.Code)
	}
	if rec := postMCP(handler, initializeRequest, map[string]string{"Authorization": "Bearer wrong"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token status = %d, want 401", rec.Code)
	}
	if backend.lastHeaders() != nil {
		t.Fatal("unauthenticated requests must not reach vmanomaly")
	}

	rec := callHealthCheck(t, handler, map[string]string{
		"Authorization": "Bearer mcp-secret",
		"X-Tenant":      "team-a",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("authenticated request status = %d: %s", rec.Code, rec.Body.String())
	}
	got := backend.lastHeaders()
	if got.Get("Authorization") != "" {
		t.Fatalf("MCP auth token was forwarded to vmanomaly: %q", got.Get("Authorization"))
	}
	if got.Get("X-Tenant") != "team-a" {
		t.Fatalf("X-Tenant = %q, want team-a", got.Get("X-Tenant"))
	}

	for _, path := range []string{"/health/liveness", "/health/readiness", "/metrics"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s status = %d, want 200 without a token", path, rec.Code)
		}
	}
}
