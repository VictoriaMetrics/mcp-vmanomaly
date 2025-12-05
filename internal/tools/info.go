package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/VictoriaMetrics-Community/mcp-vmanomaly/internal/vmanomaly"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ============================================================================
// Tool Registration Functions
// ============================================================================

// RegisterInfoTools registers all query and utility tools
func RegisterInfoTools(s *server.MCPServer, client *vmanomaly.Client) {
	// get_buildinfo tool
	getBuildinfoTool := mcp.NewTool(
		"vmanomaly_get_buildinfo",
		mcp.WithDescription("Get vmanomaly server build information including version number, build timestamp, and Go version. Use this to verify server version, check compatibility, or troubleshoot issues by confirming which version is running."),
	)
	s.AddTool(getBuildinfoTool, handleGetBuildinfo(client))

	getMetricsTool := mcp.NewTool(
		"vmanomaly_get_metrics",
		mcp.WithDescription("Get currently instant Prometheus-formatted self-monitoring metrics from vmanomaly server. Returns operational metrics including reader/writer performance, model execution stats, system info, and resource usage. Output is in standard Prometheus text exposition format suitable for scraping or monitoring analysis."),
	)
	s.AddTool(getMetricsTool, handleGetMetrics(client))
}

// ============================================================================
// Tool Handlers
// ============================================================================

func handleGetBuildinfo(client *vmanomaly.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		buildInfo, err := client.GetBuildInfo(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get build info: %v", err)), nil
		}

		responseJSON, err := json.MarshalIndent(buildInfo, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}

		resultMsg := fmt.Sprintf("vmanomaly Build Information:\n\n%s", string(responseJSON))
		return mcp.NewToolResultText(resultMsg), nil
	}
}

func handleGetMetrics(client *vmanomaly.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		metrics, err := client.Metrics(ctx, nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get metrics: %v", err)), nil
		}

		resultMsg := fmt.Sprintf("vmanomaly Prometheus Metrics:\n\n%s", metrics)
		return mcp.NewToolResultText(resultMsg), nil
	}
}
