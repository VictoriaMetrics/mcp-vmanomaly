package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/metrics"
	"github.com/mark3labs/mcp-go/server"

	"github.com/VictoriaMetrics/mcp-vmanomaly/cmd/mcp-vmanomaly/config"
)

func testConfig(t *testing.T, enabledTools, disabledTools string, disableResources bool) *config.Config {
	t.Helper()
	t.Setenv("VMANOMALY_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("VMANOMALY_BEARER_TOKEN", "")
	t.Setenv("VMANOMALY_BEARER_TOKEN_FILE", "")
	t.Setenv("MCP_ENABLED_TOOLS", enabledTools)
	t.Setenv("MCP_DISABLED_TOOLS", disabledTools)
	if disableResources {
		t.Setenv("MCP_DISABLE_RESOURCES", "true")
	} else {
		t.Setenv("MCP_DISABLE_RESOURCES", "false")
	}
	cfg, err := config.InitConfig()
	if err != nil {
		t.Fatalf("InitConfig failed: %v", err)
	}
	return cfg
}

func handleMessage(t *testing.T, mcpServer *server.MCPServer, message string) map[string]any {
	t.Helper()
	response := mcpServer.HandleMessage(context.Background(), json.RawMessage(message))
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("failed to decode response %s: %v", encoded, err)
	}
	return decoded
}

func TestRunVersionDoesNotRequireConfiguration(t *testing.T) {
	t.Setenv("VMANOMALY_ENDPOINT", "")
	var stdout, stderr bytes.Buffer
	if exitCode := run([]string{"--version"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("unexpected exit code %d: %s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), serverName+" v"+version) {
		t.Fatalf("unexpected version output: %q", stdout.String())
	}
}

func TestServerInstructionsRespectUIBusinessPolicyBoundary(t *testing.T) {
	for _, expected := range []string{
		"VMUI query state and suggest_query_config expose only the query expression and language",
		"keep data_range, detection_direction, min_dev_from_expected, and min_rel_dev_from_expected in suggest_model_config changes",
		"complete vmanomaly v1.30.2+ deployment configuration outside the VMUI suggestion flow",
		"reader.queries.<alias>",
		"model-level placement remains a compatibility fallback",
	} {
		if !strings.Contains(serverInstructions, expected) {
			t.Errorf("server instructions do not contain %q", expected)
		}
	}
}

func TestRunInvalidConfigurationFails(t *testing.T) {
	t.Setenv("VMANOMALY_ENDPOINT", "")
	t.Setenv("VMANOMALY_BEARER_TOKEN", "")
	t.Setenv("VMANOMALY_BEARER_TOKEN_FILE", "")
	var stdout, stderr bytes.Buffer
	if exitCode := run(nil, &stdout, &stderr); exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "VMANOMALY_ENDPOINT is required") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestToolPolicyFiltersDiscoveryAndRejectsDirectCalls(t *testing.T) {
	cfg := testConfig(t, "vmanomaly_search_docs,vmanomaly_health_check", "vmanomaly_health_check", true)
	mcpServer, err := newMCPServer(cfg, metrics.NewSet(), false)
	if err != nil {
		t.Fatalf("newMCPServer failed: %v", err)
	}

	listResponse := handleMessage(t, mcpServer, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	result, ok := listResponse["result"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected list response: %#v", listResponse)
	}
	toolsList, ok := result["tools"].([]any)
	if !ok || len(toolsList) != 1 {
		t.Fatalf("expected exactly one discovered tool, got %#v", result["tools"])
	}
	tool := toolsList[0].(map[string]any)
	if tool["name"] != "vmanomaly_search_docs" {
		t.Fatalf("unexpected discovered tool: %#v", tool)
	}

	callResponse := handleMessage(t, mcpServer, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"vmanomaly_health_check","arguments":{}}}`)
	if callResponse["error"] == nil {
		t.Fatalf("disabled tool call unexpectedly succeeded: %#v", callResponse)
	}
}

func TestResourcesDisabledAreNotAdvertisedAndDocsSearchStillWorks(t *testing.T) {
	cfg := testConfig(t, "vmanomaly_search_docs", "", true)
	mcpServer, err := newMCPServer(cfg, metrics.NewSet(), false)
	if err != nil {
		t.Fatalf("newMCPServer failed: %v", err)
	}

	initializeResponse := handleMessage(t, mcpServer, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
	result := initializeResponse["result"].(map[string]any)
	capabilities := result["capabilities"].(map[string]any)
	if _, exists := capabilities["resources"]; exists {
		t.Fatalf("resources capability must be absent when disabled: %#v", capabilities)
	}

	resourcesResponse := handleMessage(t, mcpServer, `{"jsonrpc":"2.0","id":2,"method":"resources/list","params":{}}`)
	if resourcesResponse["error"] == nil {
		t.Fatalf("resources/list unexpectedly succeeded: %#v", resourcesResponse)
	}

	searchResponse := handleMessage(t, mcpServer, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"vmanomaly_search_docs","arguments":{"query":"prophet seasonality","limit":2}}}`)
	if searchResponse["error"] != nil {
		t.Fatalf("documentation search failed with resources disabled: %#v", searchResponse)
	}
}

func TestHooksDoNotExposeClientOrToolArguments(t *testing.T) {
	cfg := testConfig(t, "vmanomaly_search_docs", "", true)
	metricSet := metrics.NewSet()
	mcpServer, err := newMCPServer(cfg, metricSet, false)
	if err != nil {
		t.Fatalf("newMCPServer failed: %v", err)
	}

	var logOutput bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logOutput, nil)))
	t.Cleanup(func() { slog.SetDefault(originalLogger) })

	secret := "do-not-log-this-secret"
	handleMessage(t, mcpServer, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"`+secret+`","version":"`+secret+`"}}}`)
	handleMessage(t, mcpServer, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"vmanomaly_search_docs","arguments":{"query":"`+secret+`","limit":1}}}`)

	var metricOutput bytes.Buffer
	metricSet.WritePrometheus(&metricOutput)
	if strings.Contains(metricOutput.String(), secret) {
		t.Fatalf("secret leaked into metrics: %s", metricOutput.String())
	}
	if strings.Contains(logOutput.String(), secret) {
		t.Fatalf("secret leaked into logs: %s", logOutput.String())
	}
}
