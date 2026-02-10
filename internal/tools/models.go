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
// Model Configuration Tool Arguments (Struct-based schemas)
// ============================================================================

// GetModelSchemaArgs defines arguments for get_model_schema tool
type GetModelSchemaArgs struct {
	ModelClass string `json:"model_class" jsonschema:"required,enum=zscore,enum=prophet,enum=mad,enum=holtwinters,enum=std,enum=rolling_quantile,enum=isolation_forest_univariate,enum=mad_online,enum=zscore_online,enum=quantile_online,enum=auto,description=Model type to retrieve schema for. Valid values: 'zscore' (statistical z-score) 'prophet' (Facebook Prophet for seasonality) 'mad' (Median Absolute Deviation) 'holtwinters' (triple exponential smoothing) 'std' (standard deviation) 'rolling_quantile' (quantile-based detection) 'isolation_forest_univariate' (ML-based isolation) 'mad_online' (streaming MAD) 'zscore_online' (streaming z-score) 'quantile_online' (streaming quantile) 'auto' (automatic model selection). Use vmanomaly_list_models to see all available types first."`
}

// ValidateModelConfigArgs defines arguments for validate_model_config tool
type ValidateModelConfigArgs struct {
	ModelSpec map[string]any `json:"model_spec" jsonschema:"required,description=Model configuration object to validate. Must include 'class' field specifying model type (e.g. 'prophet' 'zscore' 'holtwinters') plus model-specific parameters. Use vmanomaly_get_model_schema first to see required and optional parameters for your chosen model type. Returns validation result with normalized config or detailed error messages for invalid parameters."`
}

// ============================================================================
// Tool Registration Functions
// ============================================================================

// RegisterModelTools registers all model configuration tools
func RegisterModelTools(s *server.MCPServer, client *vmanomaly.Client) {
	listModelsTool := mcp.NewTool(
		"vmanomaly_list_models",
		mcp.WithDescription("List all available anomaly detection model types supported by vmanomaly. Returns model names that can be used in model configurations. Use this as the first step when selecting a model, then call vmanomaly_get_model_schema to see parameters for your chosen model."),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           "Vmanomaly List Models",
			ReadOnlyHint:    ptr(true),
			DestructiveHint: ptr(false),
			OpenWorldHint:   ptr(false),
		}),
	)
	s.AddTool(listModelsTool, handleListModels(client))

	getModelSchemaTool := mcp.NewTool(
		"vmanomaly_get_model_schema",
		mcp.WithDescription("Get the complete JSON schema for a specific anomaly detection model type. Returns all configuration parameters, types, validation rules, default values, and descriptions. Use this after vmanomaly_list_models to understand how to configure a specific model before calling vmanomaly_validate_model_config or vmanomaly_create_detection_task."),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           "Get Model Schema",
			ReadOnlyHint:    ptr(true),
			DestructiveHint: ptr(false),
			OpenWorldHint:   ptr(false),
		}),
		mcp.WithInputSchema[GetModelSchemaArgs](),
	)
	s.AddTool(getModelSchemaTool, mcp.NewTypedToolHandler(handleGetModelSchema(client)))

	validateModelConfigTool := mcp.NewTool(
		"vmanomaly_validate_model_config",
		mcp.WithDescription("Validate an anomaly detection model configuration before using it. Returns validation result with the normalized/validated configuration or detailed error messages if invalid. Use this after building your model config to catch configuration errors before creating a detection task with vmanomaly_create_detection_task."),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           "Validate Model Config",
			ReadOnlyHint:    ptr(true),
			DestructiveHint: ptr(false),
			OpenWorldHint:   ptr(false),
		}),
		mcp.WithInputSchema[ValidateModelConfigArgs](),
	)
	s.AddTool(validateModelConfigTool, mcp.NewTypedToolHandler(handleValidateModelConfig(client)))
}

// ============================================================================
// Tool Handlers
// ============================================================================

func handleListModels(client *vmanomaly.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Call API
		models, err := client.ListModels(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list models: %v", err)), nil
		}

		// Format response
		responseJSON, err := json.MarshalIndent(models, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}

		return mcp.NewToolResultText(string(responseJSON)), nil
	}
}

func handleGetModelSchema(client *vmanomaly.Client) func(ctx context.Context, req mcp.CallToolRequest, args GetModelSchemaArgs) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest, args GetModelSchemaArgs) (*mcp.CallToolResult, error) {
		// Call API
		schema, err := client.GetModelSchema(ctx, args.ModelClass)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get model schema: %v", err)), nil
		}

		// Format response
		responseJSON, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}

		return mcp.NewToolResultText(string(responseJSON)), nil
	}
}

func handleValidateModelConfig(client *vmanomaly.Client) func(ctx context.Context, req mcp.CallToolRequest, args ValidateModelConfigArgs) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest, args ValidateModelConfigArgs) (*mcp.CallToolResult, error) {
		// Call API
		validation, err := client.ValidateModel(ctx, args.ModelSpec)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Model validation failed: %v", err)), nil
		}

		// Format response
		responseJSON, err := json.MarshalIndent(validation, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}

		// Add helpful message
		resultMsg := fmt.Sprintf("Validation Result:\n%s\n\n", string(responseJSON))
		if validation.Valid {
			resultMsg += "✓ Model configuration is valid and ready to use!"
		} else {
			resultMsg += "✗ Model configuration is invalid. Check the errors above."
		}

		return mcp.NewToolResultText(resultMsg), nil
	}
}
