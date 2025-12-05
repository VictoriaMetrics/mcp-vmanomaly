package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/VictoriaMetrics-Community/mcp-vmanomaly/internal/vmanomaly"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ============================================================================
// Query Tool Arguments (Struct-based schemas)
// ============================================================================

// QueryMetricsArgs defines arguments for query_metrics tool
type QueryMetricsArgs struct {
	Query           string  `json:"query" jsonschema_description:"Query to execute. For VictoriaMetrics (metrics): use PromQL syntax (e.g. 'up' 'rate(http_requests_total[5m])' 'node_cpu_seconds_total'). For VictoriaLogs (logs): use LogsQL syntax (e.g. '_stream:{job=\"varlogs\"}' 'error | json'). The query language is determined by datasource_type parameter."`
	Start           float64 `json:"start,omitempty" jsonschema_description:"Query start timestamp as Unix epoch seconds (e.g. 1640995200 for 2022-01-01 00:00:00 UTC). Omit to use datasource's default time range (usually last hour or as specified by query)."`
	End             float64 `json:"end,omitempty" jsonschema_description:"Query end timestamp as Unix epoch seconds (e.g. 1641081600 for 2022-01-02 00:00:00 UTC). Omit to use current time or datasource's default. Must be greater than start if both are specified."`
	Step            string  `json:"step,omitempty" jsonschema_description:"Query resolution/step interval in Go duration format. Determines granularity of returned data points. Examples: '1s' (1 second) '1m' (1 minute) '5m' (5 minutes) '1h' (1 hour). Smaller steps return more data points but increase query cost. Default: '1s'"`
	DatasourceType  string  `json:"datasource_type,omitempty" jsonschema_description:"Type of datasource to query. Valid values: 'vm' (VictoriaMetrics for time-series metrics using PromQL) or 'vmlogs' (VictoriaLogs for log data using LogsQL). Default: 'vm'"`
	DatasourceURL   string  `json:"datasource_url,omitempty" jsonschema_description:"Full URL of the datasource including protocol and port. Example: 'http://victoriametrics:8428' or 'http://victorialogs:9428'. If omitted defaults to VMANOMALY_ENDPOINT environment variable. Must be accessible from vmanomaly server."`
	TenantID        string  `json:"tenant_id,omitempty" jsonschema_description:"Tenant identifier for multi-tenant VictoriaMetrics/VictoriaLogs deployments. Format: 'accountID' or 'accountID:projectID'. Omit for single-tenant setups."`
	NoCache         string  `json:"nocache,omitempty" jsonschema_description:"Cache bypass parameter to force fresh data retrieval. Set to any non-empty value (e.g. '1' 'true') to bypass datasource cache. Useful for testing or ensuring latest data."`
	PassAuthHeaders bool    `json:"pass_auth_headers,omitempty" jsonschema_description:"When true forwards the Authorization header from the MCP request to the datasource. Enable this when your datasource requires authentication. Default: false"`
}

// ============================================================================
// Tool Registration Functions
// ============================================================================

// RegisterQueryTools registers all query and utility tools
func RegisterQueryTools(s *server.MCPServer, client *vmanomaly.Client) {
	// query_metrics tool
	queryMetricsTool := mcp.NewTool(
		"vmanomaly_query_metrics",
		mcp.WithDescription("Execute a PromQL query against VictoriaMetrics or LogsQL query against VictoriaLogs. Returns time-series data that can be used for analysis, testing queries before creating detection tasks with vmanomaly_create_detection_task, or exploring available metrics. Use this to validate your query syntax and understand the data before setting up anomaly detection."),
		mcp.WithInputSchema[QueryMetricsArgs](),
	)
	s.AddTool(queryMetricsTool, mcp.NewTypedToolHandler(handleQueryMetrics(client)))
}

// ============================================================================
// Tool Handlers
// ============================================================================

func handleQueryMetrics(client *vmanomaly.Client) func(ctx context.Context, req mcp.CallToolRequest, args QueryMetricsArgs) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest, args QueryMetricsArgs) (*mcp.CallToolResult, error) {
		// Build request with defaults
		queryReq := &vmanomaly.QueryRequest{
			Query:           args.Query,
			Step:            "1s",
			DatasourceType:  "vm",
			PassAuthHeaders: false,
		}

		// Override defaults if provided
		if args.Step != "" {
			queryReq.Step = args.Step
		}
		if args.DatasourceType != "" {
			queryReq.DatasourceType = args.DatasourceType
		}
		if args.Start > 0 {
			queryReq.Start = &args.Start
		}
		if args.End > 0 {
			queryReq.End = &args.End
		}
		if args.TenantID != "" {
			queryReq.TenantID = &args.TenantID
		}
		if args.NoCache != "" {
			queryReq.NoCache = &args.NoCache
		}

		// Use datasource_url from args or fall back to env var
		if args.DatasourceURL != "" {
			queryReq.DatasourceURL = &args.DatasourceURL
		} else {
			envURL := os.Getenv("VMANOMALY_ENDPOINT")
			if envURL != "" {
				queryReq.DatasourceURL = &envURL
			}
		}

		// Call API
		result, err := client.Query(ctx, queryReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Query failed: %v", err)), nil
		}

		// Format response
		responseJSON, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}

		resultMsg := fmt.Sprintf("Query Result:\n\n%s", string(responseJSON))
		return mcp.NewToolResultText(resultMsg), nil
	}
}
