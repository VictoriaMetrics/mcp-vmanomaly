package tools

import (
	"context"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/vmanomaly"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ArtifactArgs is the MCP contract; the client request remains independent.
type ArtifactArgs struct {
	RequiredInputs []string           `json:"required_inputs,omitempty" jsonschema_description:"Unresolved draft fields: writer.datasource_url and/or writer.tenant_id. Use http://replace-me.invalid:8428 for an unknown writer URL and never probe it. The backend returns a required-input checklist and deployment_ready=false; draft files are not ready to deploy."`
	Config         map[string]any     `json:"config" jsonschema:"required" jsonschema_description:"Complete configuration including explicit reader, writer, models and schedulers. Use secret references rather than inline credentials."`
	Alerts         *ArtifactAlertArgs `json:"alerts,omitempty" jsonschema_description:"Optional alert policy. Omit to use backend defaults; generation does not deploy rules or test notification delivery."`
}

// ArtifactAlertArgs preserves omitted numeric fields so backend defaults apply.
type ArtifactAlertArgs struct {
	EntityLabels       []string `json:"entity_labels,omitempty" jsonschema_description:"Source-series identity labels to forward into incident links. Defaults to none, selecting the query group rather than an individual entity."`
	Threshold          *float64 `json:"threshold,omitempty" jsonschema:"exclusiveMinimum=0" jsonschema_description:"Anomaly-score threshold for the strict greater-than alert expression; positive, default 1. This is not a notification-frequency or accuracy target."`
	EvaluationSeconds  *int     `json:"evaluation_seconds,omitempty" jsonschema:"minimum=1,maximum=86400" jsonschema_description:"vmalert rule-group evaluation interval in seconds, 1 through 86400; default 60. Separate from model inference cadence."`
	PersistenceSeconds *int     `json:"persistence_seconds,omitempty" jsonschema:"minimum=0,maximum=604800" jsonschema_description:"Alert for duration in seconds, 0 through 604800; default 600. Explicit 0 disables persistence. This does not count independent anomalous observations."`
	IncidentBaseURL    string   `json:"incident_base_url,omitempty" jsonschema_description:"Externally reachable HTTP(S) vmanomaly base URL, including any path prefix, for incident annotations. Omit to generate rules without incident links; do not embed credentials."`
}

func RegisterArtifactTools(s *server.MCPServer, client *vmanomaly.Client) {
	tool := mcp.NewTool("vmanomaly_generate_preset_artifacts",
		mcp.WithDescription("Render matching vmanomaly configuration and vmalert rules from a complete configuration. Preserves query policies and model/scheduler mappings. Supply a public incident base URL for investigation annotations. Schema-valid does not mean live-tested; retain all warnings. Requires the artifact API; do not retry unsupported targets or infer writer destinations."),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: ptr(true), DestructiveHint: ptr(false), OpenWorldHint: ptr(false)}),
		mcp.WithInputSchema[ArtifactArgs]())
	s.AddTool(tool, mcp.NewTypedToolHandler(handleGenerateArtifacts(client)))
}

func handleGenerateArtifacts(client *vmanomaly.Client) func(context.Context, mcp.CallToolRequest, ArtifactArgs) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest, args ArtifactArgs) (*mcp.CallToolResult, error) {
		request := vmanomaly.ArtifactRequest{Config: args.Config, RequiredInputs: args.RequiredInputs}
		if args.Alerts != nil {
			request.Alerts = &vmanomaly.ArtifactAlertPolicy{
				EntityLabels:       args.Alerts.EntityLabels,
				Threshold:          args.Alerts.Threshold,
				EvaluationSeconds:  args.Alerts.EvaluationSeconds,
				PersistenceSeconds: args.Alerts.PersistenceSeconds,
				IncidentBaseURL:    args.Alerts.IncidentBaseURL,
			}
		}
		data, err := client.GenerateArtifacts(ctx, request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultStructured(data, string(data)), nil
	}
}
