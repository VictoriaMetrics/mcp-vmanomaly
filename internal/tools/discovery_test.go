package tools

import (
	"context"
	"encoding/json"
	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/vmanomaly"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
)

func TestDiscoveryToolPreservesStructuredResultAndErrors(t *testing.T) {
	var status atomic.Int32
	status.Store(http.StatusOK)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(int(status.Load()))
		if _, err := w.Write([]byte(`{"items":[],"completeness":"unknown","warnings":["limited"]}`)); err != nil {
			t.Error(err)
		}
	}))
	defer upstream.Close()
	s := server.NewMCPServer("test", "1")
	RegisterDiscoveryTools(s, vmanomaly.NewClient(upstream.URL, "", nil))
	tool := s.GetTool("vmanomaly_discover_metrics")
	if tool == nil || tool.Tool.Annotations.ReadOnlyHint == nil || !*tool.Tool.Annotations.ReadOnlyHint {
		t.Fatal("missing read-only tool")
	}
	var advertised struct {
		InputSchema struct {
			Required []string `json:"required"`
		} `json:"inputSchema"`
	}
	metadata, err := json.Marshal(tool.Tool)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(metadata, &advertised); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"operation", "datasource_url", "match", "start", "end"} {
		if !slices.Contains(advertised.InputSchema.Required, field) {
			t.Errorf("missing required scope field: %s", field)
		}
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
	status.Store(http.StatusNotFound)
	result, err = tool.Handler(context.Background(), request)
	if err != nil || !result.IsError {
		t.Fatal("unsupported endpoint appeared successful")
	}
}
