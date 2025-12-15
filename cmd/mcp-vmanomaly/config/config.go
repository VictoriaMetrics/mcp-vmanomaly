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
	disabledTools     map[string]bool
	heartbeatInterval time.Duration
	disableResources  bool
	logLevel          string
	logFile           string
	bearerToken       string
	customHeaders     map[string]string
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
	// Parse disabled tools
	disabledTools := os.Getenv("MCP_DISABLED_TOOLS")
	disabledToolsMap := make(map[string]bool)
	if disabledTools != "" {
		for _, tool := range strings.Split(disabledTools, ",") {
			tool = strings.Trim(tool, " ,")
			if tool != "" {
				disabledToolsMap[tool] = true
			}
		}
	}

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

	result := &Config{
		vmanomalyEndpoint: os.Getenv("VMANOMALY_ENDPOINT"),
		serverMode:        strings.ToLower(os.Getenv("MCP_SERVER_MODE")),
		listenAddr:        os.Getenv("MCP_LISTEN_ADDR"),
		disabledTools:     disabledToolsMap,
		heartbeatInterval: heartbeatInterval,
		disableResources:  disableResources,
		logLevel:          strings.ToLower(os.Getenv("MCP_LOG_LEVEL")),
		logFile:           os.Getenv("MCP_LOG_FILE"),
		bearerToken:       os.Getenv("VMANOMALY_BEARER_TOKEN"),
		customHeaders:     customHeadersMap,
	}

	// Validate required config
	if result.vmanomalyEndpoint == "" {
		return nil, fmt.Errorf("VMANOMALY_ENDPOINT is required")
	}

	// Validate server mode
	if result.serverMode != "" && result.serverMode != "stdio" && result.serverMode != "http" {
		return nil, fmt.Errorf("MCP_SERVER_MODE must be 'stdio' or 'http'")
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

func (c *Config) VmanomalyEndpoint() string {
	return c.vmanomalyEndpoint
}

func (c *Config) ServerMode() string {
	return c.serverMode
}

func (c *Config) IsStdio() bool {
	return c.serverMode == "stdio"
}

func (c *Config) IsHTTP() bool {
	return c.serverMode == "http"
}

func (c *Config) ListenAddr() string {
	return c.listenAddr
}

func (c *Config) IsToolDisabled(toolName string) bool {
	if c.disabledTools == nil {
		return false
	}
	disabled, ok := c.disabledTools[toolName]
	return ok && disabled
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
