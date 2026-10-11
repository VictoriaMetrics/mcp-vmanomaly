package tools

import (
	"context"
	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/vmanomaly"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterArtifactTools(s *server.MCPServer, client *vmanomaly.Client) {
	tool := mcp.NewTool("vmanomaly_generate_preset_artifacts",
		mcp.WithDescription("Render matching vmanomaly configuration and vmalert rules from a complete configuration. Preserves query policies and model/scheduler mappings. Supply a public incident base URL for investigation annotations. Schema-valid does not mean live-tested; retain all warnings. Requires the artifact API; do not retry unsupported targets or infer writer destinations."),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: ptr(true), DestructiveHint: ptr(false), OpenWorldHint: ptr(false)}),
		mcp.WithInputSchema[vmanomaly.ArtifactRequest]())
	s.AddTool(tool, mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args vmanomaly.ArtifactRequest) (*mcp.CallToolResult, error) {
		data, err := client.GenerateArtifacts(ctx, args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultStructured(data, string(data)), nil
	}))
}
