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
	ModelClass string `json:"model_class" jsonschema:"required,description=UI-compatible model class or alias returned by vmanomaly_list_models. Multivariate models are intentionally unavailable from this UI-oriented schema endpoint."`
}

// ValidateModelConfigArgs defines arguments for validate_model_config tool
type ValidateModelConfigArgs struct {
	ModelSpec map[string]any `json:"model_spec" jsonschema:"required,description=Model configuration object to validate. Must include 'class' plus model-specific parameters. Use vmanomaly_get_model_schema for UI-compatible univariate models. For documented multivariate models outside VMUI use documentation and this validation endpoint because UI discovery/schema intentionally hides them. Returns normalized config or detailed validation errors."`
}

// ============================================================================
// Tool Registration Functions
// ============================================================================

// RegisterModelTools registers all model configuration tools
func RegisterModelTools(s *server.MCPServer, client *vmanomaly.Client) {
	listModelsTool := mcp.NewTool(
		"vmanomaly_list_models",
		mcp.WithDescription("List model types exposed to VMUI and other UI-oriented configuration flows. Use this before selecting a model in UI, then call vmanomaly_get_model_schema. Multivariate models are intentionally omitted from this list; outside VMUI, documented multivariate aliases can still be autotuned and validated as complete model configurations."),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           "Vmanomaly List Models",
			ReadOnlyHint:    ptr(true),
			DestructiveHint: ptr(false),
			OpenWorldHint:   ptr(false),
		}),
	)
	s.AddTool(listModelsTool, handleListModels(client))

	getServerModelsTool := mcp.NewTool(
		"vmanomaly_get_server_models",
		mcp.WithDescription("Get configured server models and their query attachments. Returns runtime model aliases, model configuration, attached queries, and model metadata from the vmanomaly server."),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           "Get Server Models",
			ReadOnlyHint:    ptr(true),
			DestructiveHint: ptr(false),
			OpenWorldHint:   ptr(false),
		}),
	)
	s.AddTool(getServerModelsTool, handleGetServerModels(client))

	getModelSchemaTool := mcp.NewTool(
		"vmanomaly_get_model_schema",
		mcp.WithDescription("Get the complete JSON schema for a UI-compatible model type returned by vmanomaly_list_models. Returns parameters, types, validation rules, defaults, and descriptions. Multivariate models are intentionally unavailable from this UI-oriented endpoint; configure them outside VMUI using documentation and validate the complete model config."),
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

func handleGetServerModels(client *vmanomaly.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		models, err := client.GetServerModels(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get server models: %v", err)), nil
		}

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
