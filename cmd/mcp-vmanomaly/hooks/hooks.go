package hooks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/metrics"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func metricLabelValue(value string) string {
	return strconv.Quote(value)
}

func classifyError(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	}

	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "timeout"), strings.Contains(message, "deadline exceeded"):
		return "timeout"
	case strings.Contains(message, "not available"), strings.Contains(message, "disabled"):
		return "policy_denied"
	case strings.Contains(message, "connection refused"), strings.Contains(message, "no such host"):
		return "unavailable"
	case strings.Contains(message, "not found"):
		return "not_found"
	case strings.Contains(message, "invalid"), strings.Contains(message, "unparsable"):
		return "invalid_request"
	default:
		return "internal"
	}
}

// ErrorClass returns a bounded, non-sensitive category suitable for logs and
// metrics. It deliberately excludes the original error message.
func ErrorClass(err error) string {
	return classifyError(err)
}

func New(ms *metrics.Set) *server.Hooks {
	hooks := &server.Hooks{}

	hooks.AddAfterInitialize(func(_ context.Context, _ any, _ *mcp.InitializeRequest, _ *mcp.InitializeResult) {
		// Client-provided names and versions are intentionally not labels: an
		// unauthenticated client could otherwise create unbounded time series.
		ms.GetOrCreateCounter(`mcp_vmanomaly_initialize_total`).Inc()
	})

	hooks.AddAfterListTools(func(_ context.Context, _ any, _ *mcp.ListToolsRequest, _ *mcp.ListToolsResult) {
		ms.GetOrCreateCounter(`mcp_vmanomaly_list_tools_total`).Inc()
	})

	hooks.AddAfterListResources(func(_ context.Context, _ any, _ *mcp.ListResourcesRequest, _ *mcp.ListResourcesResult) {
		ms.GetOrCreateCounter(`mcp_vmanomaly_list_resources_total`).Inc()
	})

	hooks.AddAfterListPrompts(func(_ context.Context, _ any, _ *mcp.ListPromptsRequest, _ *mcp.ListPromptsResult) {
		ms.GetOrCreateCounter(`mcp_vmanomaly_list_prompts_total`).Inc()
	})

	hooks.AddAfterCallTool(func(_ context.Context, _ any, message *mcp.CallToolRequest, result *mcp.CallToolResult) {
		ms.GetOrCreateCounter(fmt.Sprintf(
			`mcp_vmanomaly_call_tool_total{name=%s,is_error=%s}`,
			metricLabelValue(message.Params.Name),
			metricLabelValue(strconv.FormatBool(result.IsError)),
		)).Inc()

		if result.IsError {
			slog.Warn("Tool call failed", "tool", message.Params.Name)
		} else {
			// Tool arguments and results may contain credentials, tenant IDs,
			// datasource URLs, queries, and configuration. Never log them.
			slog.Info("Tool called", "tool", message.Params.Name)
		}
	})

	hooks.AddAfterGetPrompt(func(_ context.Context, _ any, message *mcp.GetPromptRequest, _ *mcp.GetPromptResult) {
		ms.GetOrCreateCounter(fmt.Sprintf(
			`mcp_vmanomaly_get_prompt_total{name=%s}`,
			metricLabelValue(message.Params.Name),
		)).Inc()
	})

	hooks.AddAfterReadResource(func(_ context.Context, _ any, _ *mcp.ReadResourceRequest, _ *mcp.ReadResourceResult) {
		// Resource URIs are client-selected input and are deliberately omitted
		// from labels to keep cardinality bounded.
		ms.GetOrCreateCounter(`mcp_vmanomaly_read_resource_total`).Inc()
	})

	hooks.AddOnError(func(_ context.Context, _ any, method mcp.MCPMethod, _ any, err error) {
		errorClass := classifyError(err)
		ms.GetOrCreateCounter(fmt.Sprintf(
			`mcp_vmanomaly_error_total{method=%s,error_class=%s}`,
			metricLabelValue(string(method)),
			metricLabelValue(errorClass),
		)).Inc()

		// Raw protocol/API errors can echo secrets or request data. Log a
		// bounded class and keep detailed errors in the MCP response only.
		slog.Error("MCP operation error", "method", method, "error_class", errorClass)
	})

	return hooks
}
