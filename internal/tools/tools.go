package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/VictoriaMetrics-Community/mcp-vmanomaly/internal/vmanomaly"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterTools(s *server.MCPServer, client *vmanomaly.Client) {
	healthTool := mcp.NewTool("vmanomaly_health_check",
		mcp.WithDescription("Check the health status of the vmanomaly server"),
	)
	s.AddTool(healthTool, handleHealthCheck(client))

	RegisterModelTools(s, client)
	RegisterConfigTools(s, client)
	RegisterInfoTools(s, client)
	RegisterCompatibilityTools(s, client)
	RegisterAlertTools(s, client)
	RegisterDocsTool(s)
}

func handleHealthCheck(client *vmanomaly.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		health, err := client.GetHealth(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Health check failed: %v", err)), nil
		}

		responseJSON, err := json.MarshalIndent(health, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}

		return mcp.NewToolResultText(string(responseJSON)), nil
	}
}

// Example of how to add more tools:
//
// listModelsTool := mcp.NewTool("list_models",
//     mcp.WithDescription("List available anomaly detection models"),
// )
//
// s.AddTool(listModelsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
//     models, err := client.ListModels(ctx)
//     if err != nil {
//         return mcp.NewToolResultError(fmt.Sprintf("Failed to list models: %v", err)), nil
//     }
//
//     responseJSON, err := json.MarshalIndent(models, "", "  ")
//     if err != nil {
//         return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
//     }
//
//     return mcp.NewToolResultText(string(responseJSON)), nil
// })
