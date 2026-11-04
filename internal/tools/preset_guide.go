package tools

import (
	"context"
	"encoding/json"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/resources"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterPresetGuideTool exposes the guide to tool-only clients such as embedded Copilot.
func RegisterPresetGuideTool(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("vmanomaly_get_preset_guide",
		mcp.WithDescription("Retrieve the versioned workflow for building an anomaly-detection preset from discovered signals. Call before starting a preset request, including node-exporter onboarding. Covers bounded discovery, profiling, model selection, sizing, missing destinations and generating downloadable config/alert artifacts. Does not query data or modify configuration."),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: ptr(true), DestructiveHint: ptr(false), OpenWorldHint: ptr(false)})),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			guide := resources.PresetGuide()
			data, err := json.Marshal(guide)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultStructured(guide, string(data)), nil
		})
}
