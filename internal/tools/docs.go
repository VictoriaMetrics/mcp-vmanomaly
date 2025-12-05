package tools

import (
	"context"
	"fmt"
	"log"

	"github.com/VictoriaMetrics-Community/mcp-vmanomaly/internal/resources"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ============================================================================
// Documentation Search Tool Arguments (Struct-based schemas)
// ============================================================================

// SearchDocsArgs defines arguments for search_docs tool
type SearchDocsArgs struct {
	Query string  `json:"query" jsonschema_description:"Search query for vmanomaly documentation. Supports keywords phrases or natural language questions. Example queries: 'prophet model parameters' 'how to configure seasonality' 'online vs batch models' 'installation requirements' 'troubleshooting errors'. Uses fuzzy matching to find relevant documentation chunks."`
	Limit float64 `json:"limit,omitempty" jsonschema_description:"Maximum number of documentation resources to return. Range: 1-100. Default: 30. Higher limits provide more context but may include less relevant results."`
}

// ============================================================================
// Tool Registration Functions
// ============================================================================

// RegisterDocsTool registers the documentation search tool
func RegisterDocsTool(s *server.MCPServer) {
	searchDocsTool := mcp.NewTool(
		"vmanomaly_search_docs",
		mcp.WithDescription("Search vmanomaly documentation using full-text search with fuzzy matching. Returns relevant documentation resources that can help answer questions about anomaly detection, models, configuration, and vmanomaly features. Use this when you need information about model parameters, configuration syntax, troubleshooting, or feature explanations."),
		mcp.WithInputSchema[SearchDocsArgs](),
	)
	s.AddTool(searchDocsTool, mcp.NewTypedToolHandler(handleSearchDocs()))
}

// ============================================================================
// Tool Handlers
// ============================================================================

// handleSearchDocs handles the search_docs tool
func handleSearchDocs() func(ctx context.Context, req mcp.CallToolRequest, args SearchDocsArgs) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest, args SearchDocsArgs) (*mcp.CallToolResult, error) {
		// Validate and set defaults
		limit := int(args.Limit)
		if limit < 1 {
			limit = 30 // default
		}

		// Search documentation
		rs, err := resources.SearchDocResources(args.Query, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Search failed: %v", err)), nil
		}

		// Build result with embedded resources
		result := &mcp.CallToolResult{Content: []mcp.Content{}}
		for _, resource := range rs {
			content, err := resources.GetDocResourceContent(resource.URI)
			if err != nil {
				log.Printf("error getting content for resource %s: %v", resource.URI, err)
				continue
			}
			result.Content = append(result.Content, mcp.EmbeddedResource{
				Type:     "resource",
				Resource: content,
			})
		}

		if len(result.Content) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No documentation found for query: %s", args.Query)), nil
		}

		return result, nil
	}
}
