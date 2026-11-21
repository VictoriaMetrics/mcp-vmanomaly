package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/metrics"
	"github.com/mark3labs/mcp-go/server"
)

const initializeRequest = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`

// loopbackRequest simulates a request that a same-host proxy forwards over
// 127.0.0.1 while keeping the public Host header.
func loopbackRequest(method, target, body, host string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = host
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	localAddr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	return req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, localAddr))
}

func newStreamableHTTPHandler(t *testing.T, stateless, disableLocalhostProtection string) http.Handler {
	t.Helper()
	t.Setenv("MCP_STATELESS", stateless)
	t.Setenv("MCP_DISABLE_LOCALHOST_PROTECTION", disableLocalhostProtection)
	cfg := testConfig(t, "vmanomaly_search_docs", "", true)
	mcpServer, err := newMCPServer(cfg, metrics.NewSet(), false)
	if err != nil {
		t.Fatalf("newMCPServer failed: %v", err)
	}
	return server.NewStreamableHTTPServer(mcpServer, streamableHTTPOptions(cfg)...)
}

func TestStreamableHTTPLocalhostProtection(t *testing.T) {
	testCases := []struct {
		name       string
		disable    string
		host       string
		wantStatus int
	}{
		{name: "public host over loopback is rejected by default", disable: "", host: "mcp.example.com", wantStatus: http.StatusForbidden},
		{name: "localhost host is accepted by default", disable: "", host: "localhost:8080", wantStatus: http.StatusOK},
		{name: "public host over loopback is accepted when disabled", disable: "true", host: "mcp.example.com", wantStatus: http.StatusOK},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler := newStreamableHTTPHandler(t, "", tc.disable)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, loopbackRequest(http.MethodPost, "/mcp", initializeRequest, tc.host))
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestStreamableHTTPStatelessAcceptsForeignSessionIDs(t *testing.T) {
	const listTools = `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	testCases := []struct {
		name      string
		stateless string
		wantOK    bool
	}{
		{name: "gateway session id is rejected by default", stateless: "", wantOK: false},
		{name: "gateway session id is ignored in stateless mode", stateless: "true", wantOK: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler := newStreamableHTTPHandler(t, tc.stateless, "")
			req := loopbackRequest(http.MethodPost, "/mcp", listTools, "localhost:8080")
			req.Header.Set("Mcp-Session-Id", "gateway-session-1")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if gotOK := rec.Code == http.StatusOK; gotOK != tc.wantOK {
				t.Fatalf("status = %d, want OK=%t; body: %s", rec.Code, tc.wantOK, rec.Body.String())
			}
			if tc.wantOK && !strings.Contains(rec.Body.String(), "vmanomaly_search_docs") {
				t.Fatalf("tools/list did not return the enabled tool: %s", rec.Body.String())
			}
		})
	}
}

func TestSSELocalhostProtection(t *testing.T) {
	testCases := []struct {
		name       string
		disable    string
		wantStatus int
	}{
		{name: "public host over loopback is rejected by default", disable: "", wantStatus: http.StatusForbidden},
		{name: "public host over loopback is accepted when disabled", disable: "true", wantStatus: http.StatusBadRequest},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MCP_DISABLE_LOCALHOST_PROTECTION", tc.disable)
			cfg := testConfig(t, "vmanomaly_search_docs", "", true)
			mcpServer, err := newMCPServer(cfg, metrics.NewSet(), false)
			if err != nil {
				t.Fatalf("newMCPServer failed: %v", err)
			}
			srv := server.NewSSEServer(mcpServer, sseOptions(cfg)...)
			rec := httptest.NewRecorder()
			// Without a sessionId the message endpoint answers 400 once the Host check passes.
			srv.MessageHandler().ServeHTTP(rec, loopbackRequest(http.MethodPost, srv.CompleteMessagePath(), initializeRequest, "mcp.example.com"))
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}
