package tools

import (
	"context"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/vmanomaly"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// DiscoveryArgs describes the MCP contract independently of the HTTP request body.
type DiscoveryArgs struct {
	Operation       string   `json:"operation" jsonschema:"required,enum=metric_names,enum=label_names,enum=label_values,enum=series" jsonschema_description:"Metadata to discover: metric names, label names, values of a named label, or series label sets."`
	DatasourceURL   string   `json:"datasource_url" jsonschema:"required" jsonschema_description:"Authorized VictoriaMetrics datasource URL; include any required path prefix."`
	TenantID        string   `json:"tenant_id,omitempty" jsonschema_description:"Optional cluster tenant: accountID, accountID:projectID or multitenant. May be embedded in datasource_url; conflicting settings are rejected. Preserve the requested scope; never automatically broaden it or enumerate tenants."`
	Match           []string `json:"match" jsonschema:"required,minItems=1,maxItems=10" jsonschema_description:"One to ten PromQL series selectors restricting discovery; each is at most 4096 UTF-8 bytes."`
	Start           float64  `json:"start" jsonschema:"required" jsonschema_description:"Fixed window start as Unix seconds since the epoch; fractional seconds are accepted."`
	End             float64  `json:"end" jsonschema:"required" jsonschema_description:"Fixed window end as Unix seconds, greater than start and at most 31 days after it."`
	Label           string   `json:"label,omitempty" jsonschema_description:"Target label name, required and nonblank for label_values; omitted for other operations."`
	Limit           int      `json:"limit,omitempty" jsonschema:"minimum=1,maximum=1000" jsonschema_description:"Maximum returned items, 1 to 1000; omit for the server default of 100. Returned counts are not total cardinality."`
	TimeoutSeconds  float64  `json:"timeout_seconds,omitempty" jsonschema:"minimum=0,maximum=30" jsonschema_description:"Upstream metadata request timeout in seconds, up to 30; omit or use 0 for the server default of 10."`
	PassAuthHeaders bool     `json:"pass_auth_headers,omitempty" jsonschema_description:"Whether the server may forward incoming authentication headers to the datasource. Default false; enable only when credential forwarding to this datasource is authorized."`
}

func RegisterDiscoveryTools(s *server.MCPServer, client *vmanomaly.Client) {
	tool := mcp.NewTool("vmanomaly_discover_metrics",
		mcp.WithDescription("Discover scoped metric names, labels or series using one bounded request. Supply selectors and a fixed time window. Returned counts are not exact cardinality; preserve completeness and warnings. Requires the discovery API; 404 means unsupported, not an empty inventory. No automatic retries or tenant enumeration."),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: ptr(true), DestructiveHint: ptr(false), OpenWorldHint: ptr(true)}),
		mcp.WithInputSchema[DiscoveryArgs]())
	s.AddTool(tool, mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args DiscoveryArgs) (*mcp.CallToolResult, error) {
		data, err := client.Discover(ctx, vmanomaly.DiscoveryRequest{
			Operation:       args.Operation,
			DatasourceURL:   args.DatasourceURL,
			TenantID:        args.TenantID,
			Match:           args.Match,
			Start:           args.Start,
			End:             args.End,
			Label:           args.Label,
			Limit:           args.Limit,
			TimeoutSeconds:  args.TimeoutSeconds,
			PassAuthHeaders: args.PassAuthHeaders,
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultStructured(data, string(data)), nil
	}))
}
