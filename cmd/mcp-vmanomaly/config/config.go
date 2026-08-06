package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	vmanomalyEndpoint string
	serverMode        string
	listenAddr        string
	enabledTools      map[string]bool
	disabledTools     map[string]bool
	heartbeatInterval time.Duration
	disableResources  bool
	logLevel          string
	logFile           string
	bearerToken       string
	customHeaders     map[string]string
	requestTimeout    time.Duration
}

func parseToolSet(value string) map[string]bool {
	toolSet := make(map[string]bool)
	for _, tool := range strings.Split(value, ",") {
		tool = strings.TrimSpace(tool)
		if tool != "" {
			toolSet[tool] = true
		}
	}
	return toolSet
}

func parseCustomHeaders(headersEnv string) map[string]string {
	customHeadersMap := make(map[string]string)
	if headersEnv == "" {
		return customHeadersMap
	}

	for _, header := range strings.Split(headersEnv, ",") {
		header = strings.TrimSpace(header)
		if header != "" {
			parts := strings.SplitN(header, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				if key != "" && value != "" {
					customHeadersMap[key] = value
				}
			}
		}
	}

	return customHeadersMap
}

func InitConfig() (*Config, error) {
	// An empty allowlist means all tools are eligible. The denylist always wins.
	enabledToolsMap := parseToolSet(os.Getenv("MCP_ENABLED_TOOLS"))
	disabledToolsMap := parseToolSet(os.Getenv("MCP_DISABLED_TOOLS"))

	// Parse heartbeat interval
	heartbeatInterval := 30 * time.Second
	heartbeatIntervalStr := os.Getenv("MCP_HEARTBEAT_INTERVAL")
	if heartbeatIntervalStr != "" {
		interval, err := time.ParseDuration(heartbeatIntervalStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse MCP_HEARTBEAT_INTERVAL: %w", err)
		}
		if interval < 0 {
			return nil, fmt.Errorf("MCP_HEARTBEAT_INTERVAL must be non-negative")
		}
		heartbeatInterval = interval
	}

	// Parse disable resources
	disableResources := false
	disableResourcesStr := os.Getenv("MCP_DISABLE_RESOURCES")
	if disableResourcesStr != "" {
		var err error
		disableResources, err = strconv.ParseBool(disableResourcesStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse MCP_DISABLE_RESOURCES: %w", err)
		}
	}

	customHeadersMap := parseCustomHeaders(os.Getenv("VMANOMALY_HEADERS"))
	bearerToken, err := readBearerToken()
	if err != nil {
		return nil, err
	}

	requestTimeout := 30 * time.Second
	requestTimeoutStr := os.Getenv("VMANOMALY_REQUEST_TIMEOUT")
	if requestTimeoutStr != "" {
		timeout, err := time.ParseDuration(requestTimeoutStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse VMANOMALY_REQUEST_TIMEOUT: %w", err)
		}
		if timeout <= 0 {
			return nil, fmt.Errorf("VMANOMALY_REQUEST_TIMEOUT must be positive")
		}
		requestTimeout = timeout
	}

	result := &Config{
		vmanomalyEndpoint: os.Getenv("VMANOMALY_ENDPOINT"),
		serverMode:        strings.ToLower(os.Getenv("MCP_SERVER_MODE")),
		listenAddr:        os.Getenv("MCP_LISTEN_ADDR"),
		enabledTools:      enabledToolsMap,
		disabledTools:     disabledToolsMap,
		heartbeatInterval: heartbeatInterval,
		disableResources:  disableResources,
		logLevel:          strings.ToLower(os.Getenv("MCP_LOG_LEVEL")),
		logFile:           os.Getenv("MCP_LOG_FILE"),
		bearerToken:       bearerToken,
		customHeaders:     customHeadersMap,
		requestTimeout:    requestTimeout,
	}

	// Validate required config
	if result.vmanomalyEndpoint == "" {
		return nil, fmt.Errorf("VMANOMALY_ENDPOINT is required")
	}

	// Validate server mode
	if result.serverMode != "" && result.serverMode != "stdio" && result.serverMode != "sse" && result.serverMode != "http" {
		return nil, fmt.Errorf("MCP_SERVER_MODE must be 'stdio', 'sse' or 'http'")
	}

	// Validate log level
	if result.logLevel != "" && result.logLevel != "debug" && result.logLevel != "info" && result.logLevel != "warn" && result.logLevel != "error" {
		return nil, fmt.Errorf("MCP_LOG_LEVEL must be 'debug', 'info', 'warn', or 'error'")
	}

	// Default values
	if result.serverMode == "" {
		result.serverMode = "stdio"
	}
	if result.listenAddr == "" {
		result.listenAddr = "localhost:8080"
	}
	if result.logLevel == "" {
		result.logLevel = "info"
	}

	return result, nil
}

func readBearerToken() (string, error) {
	token := strings.TrimSpace(os.Getenv("VMANOMALY_BEARER_TOKEN"))
	tokenFile := strings.TrimSpace(os.Getenv("VMANOMALY_BEARER_TOKEN_FILE"))
	if token != "" && tokenFile != "" {
		return "", fmt.Errorf("VMANOMALY_BEARER_TOKEN and VMANOMALY_BEARER_TOKEN_FILE are mutually exclusive")
	}
	if tokenFile == "" {
		return token, nil
	}

	data, err := os.ReadFile(tokenFile)
	if err != nil {
		return "", fmt.Errorf("failed to read VMANOMALY_BEARER_TOKEN_FILE: %w", err)
	}
	token = strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("VMANOMALY_BEARER_TOKEN_FILE is empty")
	}
	return token, nil
}

func (c *Config) VmanomalyEndpoint() string {
	return c.vmanomalyEndpoint
}

func (c *Config) ServerMode() string {
	return c.serverMode
}

func (c *Config) IsStdio() bool {
	return c.serverMode == "stdio"
}

func (c *Config) IsSSE() bool {
	return c.serverMode == "sse"
}

func (c *Config) IsHTTP() bool {
	return c.serverMode == "http"
}

func (c *Config) ListenAddr() string {
	return c.listenAddr
}

func (c *Config) IsToolDisabled(toolName string) bool {
	return !c.IsToolEnabled(toolName)
}

// IsToolEnabled applies the configured positive allowlist and denylist.
// If MCP_ENABLED_TOOLS is empty, every registered tool is allowed unless denied.
// MCP_DISABLED_TOOLS takes precedence when a tool is present in both lists.
func (c *Config) IsToolEnabled(toolName string) bool {
	if c.disabledTools[toolName] {
		return false
	}
	if len(c.enabledTools) == 0 {
		return true
	}
	return c.enabledTools[toolName]
}

func (c *Config) IsResourcesDisabled() bool {
	return c.disableResources
}

func (c *Config) HeartbeatInterval() time.Duration {
	return c.heartbeatInterval
}

func (c *Config) LogLevel() string {
	return c.logLevel
}

func (c *Config) LogFile() string {
	return c.logFile
}

func (c *Config) BearerToken() string {
	return c.bearerToken
}

func (c *Config) CustomHeaders() map[string]string {
	return c.customHeaders
}

func (c *Config) RequestTimeout() time.Duration {
	return c.requestTimeout
}
