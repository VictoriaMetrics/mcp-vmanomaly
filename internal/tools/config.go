package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/vmanomaly"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ============================================================================
// Configuration Tool Arguments (Struct-based schemas)
// ============================================================================

// GenerateConfigArgs defines arguments for generate_config tool
type GenerateConfigArgs struct {
	Query         string         `json:"query" jsonschema:"required,description=PromQL query to monitor for anomalies"`
	Step          string         `json:"step" jsonschema:"required,description=Query step/resolution (e.g. '1m' '5m' '1h'). Use the same step used for profiling/autotune when generating a comparable final config."`
	DatasourceURL string         `json:"datasource_url" jsonschema:"required,format=uri,description=VictoriaMetrics datasource URL (e.g. 'http://victoriametrics:8428')"`
	ModelSpec     map[string]any `json:"model_spec" jsonschema:"required,description=Model specification as JSON object (must include 'class' field). For Prophet configs with step < 1h, include compression={window:'1h', agg_method:'mean', adjust_boundaries:true} unless sub-hour baseline patterns are required."`
	TenantID      string         `json:"tenant_id,omitempty" jsonschema:"description=Optional tenant ID for multi-tenancy support"`
	FitWindow     string         `json:"fit_window,omitempty" jsonschema:"description=Time window for model fitting (default: '1d')"`
	FitEvery      string         `json:"fit_every,omitempty" jsonschema:"description=Model retraining frequency (default: '1d'). For production configs choose this from drift and resource needs; do not copy the long fit_every used by controlled UI/API exact backtests."`
	InferEvery    string         `json:"infer_every,omitempty" jsonschema:"description=Optional inference cadence for batch processing. Keep equal to step unless the user asks for a different detection cadence."`
}

// ValidateConfigArgs defines arguments for validate_config tool
type ValidateConfigArgs struct {
	Config map[string]any `json:"config" jsonschema:"required,description=Complete vmanomaly configuration object to validate. Must include all required sections: 'reader' (data source) 'scheduler' (timing) 'model' (detection algorithm) and 'writer' (output destination). Returns normalized config with defaults applied or validation errors with specific issues."`
}

// ============================================================================
// Tool Registration Functions
// ============================================================================

// RegisterConfigTools registers all configuration-related tools
func RegisterConfigTools(s *server.MCPServer, client *vmanomaly.Client) {
	validateConfigTool := mcp.NewTool(
		"vmanomaly_validate_config",
		mcp.WithDescription("Validate a complete vmanomaly YAML configuration. Takes a full configuration object (with reader, scheduler, model, writer sections) and returns validation result with normalized config or error details. Use this to verify a complete config before deployment."),
		mcp.WithInputSchema[ValidateConfigArgs](),
	)
	s.AddTool(validateConfigTool, mcp.NewTypedToolHandler(handleValidateConfig(client)))
}

// ============================================================================
// Tool Handlers
// ============================================================================

// handleValidateConfig handles the validate_config tool
func handleValidateConfig(client *vmanomaly.Client) func(ctx context.Context, req mcp.CallToolRequest, args ValidateConfigArgs) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest, args ValidateConfigArgs) (*mcp.CallToolResult, error) {
		// Call API
		validation, err := client.ValidateConfig(ctx, args.Config)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Config validation failed: %v", err)), nil
		}

		// Format response
		responseJSON, err := json.MarshalIndent(validation, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}

		// Add helpful message
		resultMsg := fmt.Sprintf("Validation Result:\n%s\n\n", string(responseJSON))
		if validation.IsValid {
			resultMsg += "Configuration is valid and ready to use!"
		} else {
			resultMsg += "Configuration is invalid. Check the errors above."
		}

		return mcp.NewToolResultText(resultMsg), nil
	}
}
