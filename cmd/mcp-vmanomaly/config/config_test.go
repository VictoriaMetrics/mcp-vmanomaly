package config

import (
	"os"
	"testing"
	"time"
)

func TestInitConfig(t *testing.T) {
	// Save original environment variables
	originalEndpoint := os.Getenv("VMANOMALY_ENDPOINT")
	originalServerMode := os.Getenv("MCP_SERVER_MODE")
	originalListenAddr := os.Getenv("MCP_LISTEN_ADDR")
	originalLogLevel := os.Getenv("MCP_LOG_LEVEL")
	originalLogFile := os.Getenv("MCP_LOG_FILE")
	originalBearerToken := os.Getenv("VMANOMALY_BEARER_TOKEN")
	originalHeaders := os.Getenv("VMANOMALY_HEADERS")
	originalDisabledTools := os.Getenv("MCP_DISABLED_TOOLS")
	originalHeartbeatInterval := os.Getenv("MCP_HEARTBEAT_INTERVAL")
	originalDisableResources := os.Getenv("MCP_DISABLE_RESOURCES")

	// Restore environment variables after test
	defer func() {
		os.Setenv("VMANOMALY_ENDPOINT", originalEndpoint)
		os.Setenv("MCP_SERVER_MODE", originalServerMode)
		os.Setenv("MCP_LISTEN_ADDR", originalListenAddr)
		os.Setenv("MCP_LOG_LEVEL", originalLogLevel)
		os.Setenv("MCP_LOG_FILE", originalLogFile)
		os.Setenv("VMANOMALY_BEARER_TOKEN", originalBearerToken)
		os.Setenv("VMANOMALY_HEADERS", originalHeaders)
		os.Setenv("MCP_DISABLED_TOOLS", originalDisabledTools)
		os.Setenv("MCP_HEARTBEAT_INTERVAL", originalHeartbeatInterval)
		os.Setenv("MCP_DISABLE_RESOURCES", originalDisableResources)
	}()

	// Test case 1: Valid configuration
	t.Run("Valid configuration", func(t *testing.T) {
		// Set environment variables
		os.Setenv("VMANOMALY_ENDPOINT", "http://localhost:8490")
		os.Setenv("MCP_SERVER_MODE", "http")
		os.Setenv("MCP_LISTEN_ADDR", "localhost:9090")
		os.Setenv("MCP_LOG_LEVEL", "debug")
		os.Setenv("MCP_LOG_FILE", "/tmp/test.log")
		os.Setenv("VMANOMALY_BEARER_TOKEN", "test-token")
		os.Setenv("VMANOMALY_HEADERS", "X-Custom=value")
		os.Setenv("MCP_DISABLED_TOOLS", "tool1,tool2")
		os.Setenv("MCP_HEARTBEAT_INTERVAL", "60s")
		os.Setenv("MCP_DISABLE_RESOURCES", "true")

		// Initialize config
		cfg, err := InitConfig()

		// Check for errors
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Check config values
		if cfg.VmanomalyEndpoint() != "http://localhost:8490" {
			t.Errorf("Expected endpoint 'http://localhost:8490', got: %s", cfg.VmanomalyEndpoint())
		}
		if cfg.ServerMode() != "http" {
			t.Errorf("Expected server mode 'http', got: %s", cfg.ServerMode())
		}
		if !cfg.IsHTTP() {
			t.Error("Expected IsHTTP() to be true")
		}
		if cfg.ListenAddr() != "localhost:9090" {
			t.Errorf("Expected listen addr 'localhost:9090', got: %s", cfg.ListenAddr())
		}
		if cfg.LogLevel() != "debug" {
			t.Errorf("Expected log level 'debug', got: %s", cfg.LogLevel())
		}
		if cfg.LogFile() != "/tmp/test.log" {
			t.Errorf("Expected log file '/tmp/test.log', got: %s", cfg.LogFile())
		}
		if cfg.BearerToken() != "test-token" {
			t.Errorf("Expected bearer token 'test-token', got: %s", cfg.BearerToken())
		}
		if !cfg.IsToolDisabled("tool1") {
			t.Error("Expected tool1 to be disabled")
		}
		if !cfg.IsToolDisabled("tool2") {
			t.Error("Expected tool2 to be disabled")
		}
		if cfg.HeartbeatInterval() != 60*time.Second {
			t.Errorf("Expected heartbeat interval 60s, got: %v", cfg.HeartbeatInterval())
		}
		if !cfg.IsResourcesDisabled() {
			t.Error("Expected resources to be disabled")
		}
	})

	// Test case 2: Missing VMANOMALY_ENDPOINT
	t.Run("Missing VMANOMALY_ENDPOINT", func(t *testing.T) {
		// Clear environment variables
		os.Setenv("VMANOMALY_ENDPOINT", "")

		// Initialize config
		_, err := InitConfig()

		// Check for errors
		if err == nil {
			t.Fatal("Expected error for missing VMANOMALY_ENDPOINT, got nil")
		}
	})

	// Test case 3: Invalid server mode
	t.Run("Invalid server mode", func(t *testing.T) {
		// Set environment variables
		os.Setenv("VMANOMALY_ENDPOINT", "http://localhost:8490")
		os.Setenv("MCP_SERVER_MODE", "invalid")

		// Initialize config
		_, err := InitConfig()

		// Check for errors
		if err == nil {
			t.Fatal("Expected error for invalid server mode, got nil")
		}
	})

	// Test case 4: Invalid log level
	t.Run("Invalid log level", func(t *testing.T) {
		// Set environment variables
		os.Setenv("VMANOMALY_ENDPOINT", "http://localhost:8490")
		os.Setenv("MCP_LOG_LEVEL", "invalid")

		// Initialize config
		_, err := InitConfig()

		// Check for errors
		if err == nil {
			t.Fatal("Expected error for invalid log level, got nil")
		}
	})

	// Test case 5: Default values
	t.Run("Default values", func(t *testing.T) {
		// Set only required variables
		os.Setenv("VMANOMALY_ENDPOINT", "http://localhost:8490")
		os.Setenv("MCP_SERVER_MODE", "")
		os.Setenv("MCP_LISTEN_ADDR", "")
		os.Setenv("MCP_LOG_LEVEL", "")
		os.Setenv("MCP_LOG_FILE", "")
		os.Setenv("VMANOMALY_BEARER_TOKEN", "")
		os.Setenv("VMANOMALY_HEADERS", "")
		os.Setenv("MCP_DISABLED_TOOLS", "")
		os.Setenv("MCP_HEARTBEAT_INTERVAL", "")
		os.Setenv("MCP_DISABLE_RESOURCES", "")

		// Initialize config
		cfg, err := InitConfig()

		// Check for errors
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Check default values
		if !cfg.IsStdio() {
			t.Error("Expected default server mode to be stdio")
		}
		if cfg.ListenAddr() != "localhost:8080" {
			t.Errorf("Expected default listen addr 'localhost:8080', got: %s", cfg.ListenAddr())
		}
		if cfg.LogLevel() != "info" {
			t.Errorf("Expected default log level 'info', got: %s", cfg.LogLevel())
		}
		if cfg.HeartbeatInterval() != 30*time.Second {
			t.Errorf("Expected default heartbeat interval 30s, got: %v", cfg.HeartbeatInterval())
		}
		if cfg.IsResourcesDisabled() {
			t.Error("Expected resources to be enabled by default")
		}
	})

	// Test case 6: Valid heartbeat interval
	t.Run("Valid heartbeat interval", func(t *testing.T) {
		// Set environment variables
		os.Setenv("VMANOMALY_ENDPOINT", "http://localhost:8490")
		os.Setenv("MCP_HEARTBEAT_INTERVAL", "45s")

		// Initialize config
		cfg, err := InitConfig()

		// Check for errors
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Check heartbeat interval
		if cfg.HeartbeatInterval() != 45*time.Second {
			t.Errorf("Expected heartbeat interval 45s, got: %v", cfg.HeartbeatInterval())
		}
	})

	// Test case 7: Invalid heartbeat interval
	t.Run("Invalid heartbeat interval", func(t *testing.T) {
		// Set environment variables
		os.Setenv("VMANOMALY_ENDPOINT", "http://localhost:8490")
		os.Setenv("MCP_HEARTBEAT_INTERVAL", "invalid")

		// Initialize config
		_, err := InitConfig()

		// Check for errors
		if err == nil {
			t.Fatal("Expected error for invalid heartbeat interval, got nil")
		}
	})

	// Test case 8: Custom headers parsing
	t.Run("Custom headers parsing", func(t *testing.T) {
		// Reset environment variables
		os.Setenv("MCP_HEARTBEAT_INTERVAL", "")

		// Set environment variables
		os.Setenv("VMANOMALY_ENDPOINT", "http://localhost:8490")
		os.Setenv("VMANOMALY_HEADERS", "X-Auth-Token=secret123,X-Custom-Header=value,X-Another=test")

		// Initialize config
		cfg, err := InitConfig()

		// Check for errors
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Check custom headers
		headers := cfg.CustomHeaders()
		expectedHeaders := map[string]string{
			"X-Auth-Token":    "secret123",
			"X-Custom-Header": "value",
			"X-Another":       "test",
		}

		if len(headers) != len(expectedHeaders) {
			t.Errorf("Expected %d headers, got %d", len(expectedHeaders), len(headers))
		}

		for key, expectedValue := range expectedHeaders {
			if actualValue, exists := headers[key]; !exists {
				t.Errorf("Expected header %s to exist", key)
			} else if actualValue != expectedValue {
				t.Errorf("Expected header %s to have value %s, got %s", key, expectedValue, actualValue)
			}
		}
	})

	// Test case 9: Empty custom headers
	t.Run("Empty custom headers", func(t *testing.T) {
		// Reset environment variables
		os.Setenv("MCP_HEARTBEAT_INTERVAL", "")

		// Set environment variables
		os.Setenv("VMANOMALY_ENDPOINT", "http://localhost:8490")
		os.Setenv("VMANOMALY_HEADERS", "")

		// Initialize config
		cfg, err := InitConfig()

		// Check for errors
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Check custom headers
		headers := cfg.CustomHeaders()
		if len(headers) != 0 {
			t.Errorf("Expected 0 headers, got %d", len(headers))
		}
	})

	// Test case 10: Invalid header format (should be ignored)
	t.Run("Invalid header format", func(t *testing.T) {
		// Reset environment variables
		os.Setenv("MCP_HEARTBEAT_INTERVAL", "")

		// Set environment variables
		os.Setenv("VMANOMALY_ENDPOINT", "http://localhost:8490")
		os.Setenv("VMANOMALY_HEADERS", "invalid-header,valid-header=value,another-invalid")

		// Initialize config
		cfg, err := InitConfig()

		// Check for errors
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Check custom headers (only valid ones should be parsed)
		headers := cfg.CustomHeaders()
		expectedHeaders := map[string]string{
			"valid-header": "value",
		}

		if len(headers) != len(expectedHeaders) {
			t.Errorf("Expected %d headers, got %d", len(expectedHeaders), len(headers))
		}

		for key, expectedValue := range expectedHeaders {
			if actualValue, exists := headers[key]; !exists {
				t.Errorf("Expected header %s to exist", key)
			} else if actualValue != expectedValue {
				t.Errorf("Expected header %s to have value %s, got %s", key, expectedValue, actualValue)
			}
		}
	})

	// Test case 11: Bearer token
	t.Run("Bearer token", func(t *testing.T) {
		// Reset environment variables
		os.Setenv("MCP_HEARTBEAT_INTERVAL", "")

		// Set environment variables
		os.Setenv("VMANOMALY_ENDPOINT", "http://localhost:8490")
		os.Setenv("VMANOMALY_BEARER_TOKEN", "my-secret-token-123")

		// Initialize config
		cfg, err := InitConfig()

		// Check for errors
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Check bearer token
		if cfg.BearerToken() != "my-secret-token-123" {
			t.Errorf("Expected bearer token 'my-secret-token-123', got: %s", cfg.BearerToken())
		}
	})

	// Test case 12: Disabled tools
	t.Run("Disabled tools", func(t *testing.T) {
		// Reset environment variables
		os.Setenv("MCP_HEARTBEAT_INTERVAL", "")

		// Set environment variables
		os.Setenv("VMANOMALY_ENDPOINT", "http://localhost:8490")
		os.Setenv("MCP_DISABLED_TOOLS", "health_check,search_docs,list_models")

		// Initialize config
		cfg, err := InitConfig()

		// Check for errors
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Check disabled tools
		if !cfg.IsToolDisabled("health_check") {
			t.Error("Expected health_check to be disabled")
		}
		if !cfg.IsToolDisabled("search_docs") {
			t.Error("Expected search_docs to be disabled")
		}
		if !cfg.IsToolDisabled("list_models") {
			t.Error("Expected list_models to be disabled")
		}
		if cfg.IsToolDisabled("query_metrics") {
			t.Error("Expected query_metrics to be enabled")
		}
	})

	// Test case 13: Disable resources
	t.Run("Disable resources", func(t *testing.T) {
		// Reset environment variables
		os.Setenv("MCP_HEARTBEAT_INTERVAL", "")

		// Set environment variables
		os.Setenv("VMANOMALY_ENDPOINT", "http://localhost:8490")
		os.Setenv("MCP_DISABLE_RESOURCES", "true")

		// Initialize config
		cfg, err := InitConfig()

		// Check for errors
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Check disable resources
		if !cfg.IsResourcesDisabled() {
			t.Error("Expected resources to be disabled")
		}
	})

	// Test case 14: Server mode variations
	t.Run("Server mode variations", func(t *testing.T) {
		// Reset environment variables
		os.Setenv("MCP_HEARTBEAT_INTERVAL", "")

		// Test stdio mode
		os.Setenv("VMANOMALY_ENDPOINT", "http://localhost:8490")
		os.Setenv("MCP_SERVER_MODE", "stdio")
		cfg, err := InitConfig()
		if err != nil {
			t.Fatalf("Expected no error for stdio mode, got: %v", err)
		}
		if !cfg.IsStdio() {
			t.Error("Expected IsStdio() to be true")
		}

		// Test http mode
		os.Setenv("MCP_SERVER_MODE", "http")
		cfg, err = InitConfig()
		if err != nil {
			t.Fatalf("Expected no error for http mode, got: %v", err)
		}
		if !cfg.IsHTTP() {
			t.Error("Expected IsHTTP() to be true")
		}
	})
}
