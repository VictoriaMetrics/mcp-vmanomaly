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

func TestArtifactToolPreservesStructuredResultAndErrors(t *testing.T) {
	status := http.StatusOK
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		if _, err := w.Write([]byte(`{"schema_version":1,"validation":"schema-valid","warnings":["not live-tested"]}`)); err != nil {
			t.Error(err)
		}
	}))
	defer upstream.Close()
	s := server.NewMCPServer("test", "1")
	RegisterArtifactTools(s, vmanomaly.NewClient(upstream.URL, "", nil))
	tool := s.GetTool("vmanomaly_generate_preset_artifacts")
	if tool == nil || tool.Tool.Annotations.ReadOnlyHint == nil || !*tool.Tool.Annotations.ReadOnlyHint {
		t.Fatal("missing read-only tool")
	}
	var request mcp.CallToolRequest
	request.Params.Arguments = map[string]any{"config": map[string]any{"models": map[string]any{}}}
	result, err := tool.Handler(context.Background(), request)
	if err != nil || result.IsError {
		t.Fatalf("%v %v", result, err)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil || string(encoded) != `{"schema_version":1,"validation":"schema-valid","warnings":["not live-tested"]}` {
		t.Fatalf("structured response changed: %s %v", encoded, err)
	}
	status = http.StatusNotFound
	result, err = tool.Handler(context.Background(), request)
	if err != nil || !result.IsError {
		t.Fatal("unsupported endpoint appeared successful")
	}
}
