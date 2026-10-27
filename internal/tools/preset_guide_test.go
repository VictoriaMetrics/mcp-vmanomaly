package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/resources"
	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/vmanomaly"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestPresetGuideAvailableWithoutBackendOrDocsInitialization(t *testing.T) {
	s := server.NewMCPServer("test", "1")
	RegisterTools(s, vmanomaly.NewClient("http://unused.invalid", "", nil))
	tool := s.GetTool("vmanomaly_get_preset_guide")
	if tool == nil || tool.Tool.Annotations.ReadOnlyHint == nil || !*tool.Tool.Annotations.ReadOnlyHint {
		t.Fatal("missing read-only guide tool")
	}
	result, err := tool.Handler(context.Background(), mcp.CallToolRequest{})
	if err != nil || result.IsError {
		t.Fatalf("offline guide retrieval: %v %v", result, err)
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("tool-only clients need readable content")
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(text.Text), &body); err != nil {
		t.Fatal(err)
	}
	if body["uri"] != resources.PresetGuideURI || body["guide"] != resources.PresetGuide()["guide"] {
		t.Fatal("tool returned a different workflow")
	}
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil || string(structured) != text.Text {
		t.Fatal("structured and text guide responses differ")
	}
}
