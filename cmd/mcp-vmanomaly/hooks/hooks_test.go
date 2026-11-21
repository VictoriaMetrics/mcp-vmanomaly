package hooks

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestErrorClass(t *testing.T) {
	testCases := []struct {
		name string
		err  error
		want string
	}{
		{name: "none", err: nil, want: "none"},
		{name: "canceled", err: context.Canceled, want: "canceled"},
		{name: "deadline", err: context.DeadlineExceeded, want: "timeout"},
		{name: "policy", err: errors.New("requested tool is not available"), want: "policy_denied"},
		{name: "connection", err: errors.New("dial tcp: connection refused"), want: "unavailable"},
		{name: "invalid", err: errors.New("invalid request"), want: "invalid_request"},
		{name: "other", err: errors.New("secret internal detail"), want: "internal"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := ErrorClass(testCase.err); got != testCase.want {
				t.Fatalf("ErrorClass() = %q; want %q", got, testCase.want)
			}
		})
	}
}

func TestMetricLabelValueEscapesUntrustedInput(t *testing.T) {
	got := metricLabelValue("value\"}\nmalicious_metric 1")
	want := `"value\"}\nmalicious_metric 1"`
	if got != want {
		t.Fatalf("metricLabelValue() = %q; want %q", got, want)
	}
}

func TestMergeKeepsEveryHookKind(t *testing.T) {
	// Fill every hook slice of two Hooks values with one no-op callback each.
	fill := func() *server.Hooks {
		h := &server.Hooks{}
		v := reflect.ValueOf(h).Elem()
		for i := range v.NumField() {
			field := v.Field(i)
			if field.Kind() != reflect.Slice {
				continue
			}
			fn := reflect.MakeFunc(field.Type().Elem(), func([]reflect.Value) []reflect.Value { return nil })
			field.Set(reflect.Append(field, fn))
		}
		return h
	}

	merged := Merge(fill(), nil, fill())
	v := reflect.ValueOf(merged).Elem()
	for i := range v.NumField() {
		if field := v.Field(i); field.Kind() == reflect.Slice && field.Len() != 2 {
			t.Errorf("%s has %d hooks after merge, want 2", v.Type().Field(i).Name, field.Len())
		}
	}
}

func TestLoggerHooksAddPayloadsOnlyAtDebug(t *testing.T) {
	const secret = "do-not-log-at-info"
	request := &mcp.CallToolRequest{}
	request.Params.Name = "vmanomaly_health_check"
	request.Params.Arguments = map[string]any{"query": secret}
	callErr := errors.New("upstream said " + secret)

	for _, tc := range []struct {
		level      slog.Level
		wantSecret bool
	}{
		{level: slog.LevelInfo, wantSecret: false},
		{level: slog.LevelDebug, wantSecret: true},
	} {
		t.Run(tc.level.String(), func(t *testing.T) {
			var out bytes.Buffer
			original := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&out, &slog.HandlerOptions{Level: tc.level})))
			t.Cleanup(func() { slog.SetDefault(original) })

			h := NewLoggerHooks()
			ctx := context.Background()
			for _, hook := range h.OnBeforeAny {
				hook(ctx, 1, mcp.MethodToolsCall, request)
			}
			for _, hook := range h.OnSuccess {
				hook(ctx, 1, mcp.MethodToolsCall, request, mcp.NewToolResultText(secret))
			}
			for _, hook := range h.OnError {
				hook(ctx, 2, mcp.MethodToolsCall, request, callErr)
			}
			initialize := &mcp.InitializeRequest{}
			initialize.Params.ClientInfo.Name = secret
			for _, hook := range h.OnAfterInitialize {
				hook(ctx, 3, initialize, &mcp.InitializeResult{})
			}
			for _, hook := range h.OnAfterCallTool {
				hook(ctx, 1, request, mcp.NewToolResultError("failed"))
			}

			logs := out.String()
			if got := strings.Contains(logs, secret); got != tc.wantSecret {
				t.Fatalf("secret in logs = %t, want %t:\n%s", got, tc.wantSecret, logs)
			}
			for _, want := range []string{`"msg":"MCP request received"`, `"msg":"MCP request failed"`, `"error_class":"internal"`, `"msg":"Client initialized"`, `"tool":"vmanomaly_health_check"`, `"is_error":true`} {
				if !strings.Contains(logs, want) {
					t.Errorf("logs do not contain %s:\n%s", want, logs)
				}
			}
		})
	}
}
