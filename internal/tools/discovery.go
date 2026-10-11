package tools

import (
	"context"
	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/vmanomaly"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterDiscoveryTools(s *server.MCPServer, client *vmanomaly.Client) {
	tool := mcp.NewTool("vmanomaly_discover_metrics",
		mcp.WithDescription("Discover scoped metric names, labels or series using one bounded request. Supply selectors and a fixed time window. Returned counts are not exact cardinality; preserve completeness and warnings. Requires the discovery API; 404 means unsupported, not an empty inventory. No automatic retries or tenant enumeration."),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: ptr(true), DestructiveHint: ptr(false), OpenWorldHint: ptr(true)}),
		mcp.WithInputSchema[vmanomaly.DiscoveryRequest]())
	s.AddTool(tool, mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args vmanomaly.DiscoveryRequest) (*mcp.CallToolResult, error) {
		data, err := client.Discover(ctx, args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultStructured(data, string(data)), nil
	}))
}
