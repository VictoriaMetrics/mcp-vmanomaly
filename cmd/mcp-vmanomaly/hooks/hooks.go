package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
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

// callToolIsError reports whether a tools/call result is an error result.
// Since mcp-go v1 the hook receives the result as any, because a task-augmented
// call returns a task handle instead of *mcp.CallToolResult.
func callToolIsError(result any) bool {
	r, ok := result.(*mcp.CallToolResult)
	return ok && r != nil && r.IsError
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

	hooks.AddAfterCallTool(func(_ context.Context, _ any, message *mcp.CallToolRequest, result any) {
		isError := callToolIsError(result)
		ms.GetOrCreateCounter(fmt.Sprintf(
			`mcp_vmanomaly_call_tool_total{name=%s,is_error=%s}`,
			metricLabelValue(message.Params.Name),
			metricLabelValue(strconv.FormatBool(isError)),
		)).Inc()
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
	})

	return hooks
}

// NewLoggerHooks returns hooks that log MCP sessions, requests and tool calls.
//
// At info level and above they log only bounded, non-sensitive fields: request and
// session IDs, method, tool name and error class. Tool arguments and results can
// contain credentials, tenant IDs, datasource URLs, queries and configuration, and
// raw errors and client metadata can echo them, so request and result payloads,
// raw errors and client name/version are added only when debug logging is enabled.
func NewLoggerHooks() *server.Hooks {
	hooks := &server.Hooks{}

	hooks.AddOnRegisterSession(func(_ context.Context, session server.ClientSession) {
		slog.Info("Session registered", "session_id", session.SessionID())
	})

	hooks.AddOnUnregisterSession(func(_ context.Context, session server.ClientSession) {
		slog.Info("Session unregistered", "session_id", session.SessionID())
	})

	hooks.AddBeforeAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any) {
		slog.Info("MCP request received", withDebug(ctx, []any{
			"request_id", id,
			"session_id", extractSessionID(ctx),
			"method", string(method),
		}, "message", toJSON(message))...)
	})

	hooks.AddOnSuccess(func(ctx context.Context, id any, method mcp.MCPMethod, message any, result any) {
		slog.Info("MCP request succeeded", withDebug(ctx, []any{
			"request_id", id,
			"session_id", extractSessionID(ctx),
			"method", string(method),
		}, "message", toJSON(message), "result", toJSON(result))...)
	})

	hooks.AddOnError(func(ctx context.Context, id any, method mcp.MCPMethod, message any, err error) {
		slog.Error("MCP request failed", withDebug(ctx, []any{
			"request_id", id,
			"session_id", extractSessionID(ctx),
			"method", string(method),
			"error_class", classifyError(err),
		}, "error", errorText(err), "message", toJSON(message))...)
	})

	hooks.AddAfterInitialize(func(ctx context.Context, id any, message *mcp.InitializeRequest, _ *mcp.InitializeResult) {
		slog.Info("Client initialized", withDebug(ctx, []any{
			"request_id", id,
			"session_id", extractSessionID(ctx),
			"protocol_version", message.Params.ProtocolVersion,
		}, "client_name", message.Params.ClientInfo.Name, "client_version", message.Params.ClientInfo.Version)...)
	})

	hooks.AddAfterCallTool(func(ctx context.Context, id any, message *mcp.CallToolRequest, result any) {
		isError := callToolIsError(result)
		level := slog.LevelInfo
		if isError {
			level = slog.LevelWarn
		}
		slog.Log(ctx, level, "Tool called",
			"request_id", id,
			"session_id", extractSessionID(ctx),
			"tool", message.Params.Name,
			"is_error", isError,
		)
	})

	return hooks
}

// Merge combines hooks so that every registered callback runs, in argument order.
// It merges all hook kinds of server.Hooks, including ones added in later mcp-go versions.
func Merge(hooksList ...*server.Hooks) *server.Hooks {
	combined := &server.Hooks{}
	dst := reflect.ValueOf(combined).Elem()
	for _, h := range hooksList {
		if h == nil {
			continue
		}
		src := reflect.ValueOf(h).Elem()
		for i := range dst.NumField() {
			if field := dst.Field(i); field.Kind() == reflect.Slice && field.CanSet() {
				field.Set(reflect.AppendSlice(field, src.Field(i)))
			}
		}
	}
	return combined
}

func extractSessionID(ctx context.Context) string {
	if session := server.ClientSessionFromContext(ctx); session != nil {
		return session.SessionID()
	}
	return ""
}

// withDebug appends the key/value pairs in debugKVs to args only when debug logging is
// enabled. Payloads can be large (resources/list is megabytes) and sensitive, so at
// info level they are neither marshaled nor logged; pass values through toJSON or
// errorText so that the work happens only after this check.
func withDebug(ctx context.Context, args []any, debugKVs ...any) []any {
	if !slog.Default().Enabled(ctx, slog.LevelDebug) {
		return args
	}
	for i := 0; i+1 < len(debugKVs); i += 2 {
		value := debugKVs[i+1]
		if lazy, ok := value.(lazyValue); ok {
			value = lazy()
		}
		args = append(args, debugKVs[i], value)
	}
	return args
}

// lazyValue defers payload encoding until withDebug knows debug logging is enabled.
type lazyValue func() string

// toJSON returns a lazily encoded JSON form of v for debug logs.
func toJSON(v any) lazyValue {
	return func() string {
		if v == nil {
			return ""
		}
		b, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

func errorText(err error) lazyValue {
	return func() string {
		if err == nil {
			return ""
		}
		return err.Error()
	}
}
