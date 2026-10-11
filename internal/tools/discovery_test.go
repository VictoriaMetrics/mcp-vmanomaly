package tools

import (
	"context"
	"encoding/json"
	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/vmanomaly"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoveryToolPreservesStructuredResultAndErrors(t *testing.T) {
	status := http.StatusOK
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(`{"items":[],"completeness":"unknown","warnings":["limited"]}`))
	}))
	defer upstream.Close()
	s := server.NewMCPServer("test", "1")
	RegisterDiscoveryTools(s, vmanomaly.NewClient(upstream.URL, "", nil))
	tool := s.GetTool("vmanomaly_discover_metrics")
	if tool == nil || tool.Tool.Annotations.ReadOnlyHint == nil || !*tool.Tool.Annotations.ReadOnlyHint {
		t.Fatal("missing read-only tool")
	}
	var request mcp.CallToolRequest
	request.Params.Arguments = map[string]any{"operation": "series", "match": []string{"up"}, "start": 1, "end": 2}
	result, err := tool.Handler(context.Background(), request)
	if err != nil || result.IsError {
		t.Fatalf("%v %v", result, err)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil || string(encoded) != `{"items":[],"completeness":"unknown","warnings":["limited"]}` {
		t.Fatalf("structured response changed: %s %v", encoded, err)
	}
	status = http.StatusNotFound
	result, err = tool.Handler(context.Background(), request)
	if err != nil || !result.IsError {
		t.Fatal("unsupported endpoint appeared successful")
	}
}
