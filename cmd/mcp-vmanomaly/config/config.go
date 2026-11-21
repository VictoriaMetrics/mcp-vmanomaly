package config

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/tlsfiles"
)

type Config struct {
	vmanomalyEndpoint string
	serverMode        string
	listenAddr        string
	enabledTools      map[string]bool
	disabledTools     map[string]bool
	heartbeatInterval time.Duration
	disableResources  bool
	stateless         bool
	// disableLocalhostProtection turns off DNS-rebinding protection for loopback connections
	disableLocalhostProtection bool
	logLevel                   string
	logFormat                  string
	instructionsOverride       string
	logFile                    string
	bearerToken                string
	customHeaders              map[string]string
	// passthroughHeaders lists canonical header names copied from each MCP request to vmanomaly
	passthroughHeaders []string
	// authToken is the bearer token MCP clients must present in sse and http modes
	authToken      string
	requestTimeout time.Duration
	tlsConfig      *tls.Config
	tlsCAFile      string
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

	disableResources, err := parseBoolEnv("MCP_DISABLE_RESOURCES")
	if err != nil {
		return nil, err
	}
	stateless, err := parseBoolEnv("MCP_STATELESS")
	if err != nil {
		return nil, err
	}
	disableLocalhostProtection, err := parseBoolEnv("MCP_DISABLE_LOCALHOST_PROTECTION")
	if err != nil {
		return nil, err
	}

	customHeadersMap := parseCustomHeaders(os.Getenv("VMANOMALY_HEADERS"))
	bearerToken, err := readSecret("VMANOMALY_BEARER_TOKEN", "VMANOMALY_BEARER_TOKEN_FILE")
	if err != nil {
		return nil, err
	}
	authToken, err := readSecret("MCP_AUTH_TOKEN", "MCP_AUTH_TOKEN_FILE")
	if err != nil {
		return nil, err
	}
	if raw, set := os.LookupEnv("MCP_AUTH_TOKEN"); set && raw != "" && authToken == "" {
		// A whitespace-only value, e.g. from an unset template variable, must not silently disable auth.
		return nil, fmt.Errorf("MCP_AUTH_TOKEN is blank; unset it to disable authentication")
	}
	passthroughHeaders, err := parsePassthroughHeaders(os.Getenv("MCP_PASSTHROUGH_HEADERS"))
	if err != nil {
		return nil, err
	}
	if authToken != "" && slices.Contains(passthroughHeaders, "Authorization") {
		// The incoming Authorization header carries MCP_AUTH_TOKEN, which must not reach vmanomaly.
		return nil, fmt.Errorf("MCP_PASSTHROUGH_HEADERS cannot include Authorization when MCP_AUTH_TOKEN is set")
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

	tlsConfig, tlsCAFile, err := readTLSConfig()
	if err != nil {
		return nil, err
	}

	result := &Config{
		vmanomalyEndpoint:          os.Getenv("VMANOMALY_ENDPOINT"),
		serverMode:                 strings.ToLower(os.Getenv("MCP_SERVER_MODE")),
		listenAddr:                 os.Getenv("MCP_LISTEN_ADDR"),
		enabledTools:               enabledToolsMap,
		disabledTools:              disabledToolsMap,
		heartbeatInterval:          heartbeatInterval,
		disableResources:           disableResources,
		stateless:                  stateless,
		disableLocalhostProtection: disableLocalhostProtection,
		logLevel:                   strings.ToLower(os.Getenv("MCP_LOG_LEVEL")),
		logFormat:                  strings.ToLower(strings.TrimSpace(os.Getenv("MCP_LOG_FORMAT"))),
		instructionsOverride:       strings.TrimSpace(os.Getenv("MCP_INSTRUCTIONS_OVERRIDE")),
		logFile:                    os.Getenv("MCP_LOG_FILE"),
		bearerToken:                bearerToken,
		customHeaders:              customHeadersMap,
		passthroughHeaders:         passthroughHeaders,
		authToken:                  authToken,
		requestTimeout:             requestTimeout,
		tlsConfig:                  tlsConfig,
		tlsCAFile:                  tlsCAFile,
	}

	// Validate required config
	if result.vmanomalyEndpoint == "" {
		return nil, fmt.Errorf("VMANOMALY_ENDPOINT is required")
	}

	if tlsConfig != nil {
		endpoint, err := url.Parse(result.vmanomalyEndpoint)
		if err != nil || endpoint.Scheme != "https" {
			return nil, fmt.Errorf("TLS settings require an https VMANOMALY_ENDPOINT")
		}
	}

	// Validate server mode
	if result.serverMode != "" && result.serverMode != "stdio" && result.serverMode != "sse" && result.serverMode != "http" {
		return nil, fmt.Errorf("MCP_SERVER_MODE must be 'stdio', 'sse' or 'http'")
	}

	// Validate log level
	if result.logLevel != "" && result.logLevel != "debug" && result.logLevel != "info" && result.logLevel != "warn" && result.logLevel != "error" {
		return nil, fmt.Errorf("MCP_LOG_LEVEL must be 'debug', 'info', 'warn', or 'error'")
	}

	if result.logFormat != "" && result.logFormat != "text" && result.logFormat != "json" {
		return nil, fmt.Errorf("MCP_LOG_FORMAT must be 'text' or 'json'")
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
	if result.logFormat == "" {
		result.logFormat = "text"
	}

	return result, nil
}

// parseBoolEnv parses an optional boolean environment variable; unset or empty means false.
func parseBoolEnv(name string) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("failed to parse %s: %w", name, err)
	}
	return parsed, nil
}

// reservedPassthroughHeaders are set by the HTTP client or describe the message
// framing of each vmanomaly request, so they cannot be forwarded from MCP clients.
var reservedPassthroughHeaders = map[string]bool{
	"Connection":        true,
	"Content-Length":    true,
	"Content-Type":      true,
	"Host":              true,
	"Keep-Alive":        true,
	"Proxy-Connection":  true,
	"Te":                true,
	"Trailer":           true,
	"Transfer-Encoding": true,
	"Upgrade":           true,
}

// parsePassthroughHeaders parses a comma-separated list of HTTP header names
// and returns them in canonical form without duplicates.
func parsePassthroughHeaders(value string) ([]string, error) {
	var headers []string
	for _, name := range strings.Split(value, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if !isHeaderToken(name) {
			return nil, fmt.Errorf("MCP_PASSTHROUGH_HEADERS contains an invalid header name %q", name)
		}
		name = http.CanonicalHeaderKey(name)
		if reservedPassthroughHeaders[name] {
			return nil, fmt.Errorf("MCP_PASSTHROUGH_HEADERS cannot include %s; the server sets it for each vmanomaly request", name)
		}
		if !slices.Contains(headers, name) {
			headers = append(headers, name)
		}
	}
	return headers, nil
}

// isHeaderToken reports whether name is a valid HTTP header field name (RFC 9110 token).
func isHeaderToken(name string) bool {
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		if !strings.ContainsRune("!#$%&'*+-.^_`|~", r) {
			return false
		}
	}
	return name != ""
}

// readSecret reads a secret from the variable valueEnv or from the file named by fileEnv.
// The two sources are mutually exclusive; an unset secret is returned as "".
func readSecret(valueEnv, fileEnv string) (string, error) {
	value := strings.TrimSpace(os.Getenv(valueEnv))
	file := strings.TrimSpace(os.Getenv(fileEnv))
	if value != "" && file != "" {
		return "", fmt.Errorf("%s and %s are mutually exclusive", valueEnv, fileEnv)
	}
	if file == "" {
		return value, nil
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", fileEnv, err)
	}
	value = strings.TrimSpace(string(data))
	if value == "" {
		return "", fmt.Errorf("%s is empty", fileEnv)
	}
	return value, nil
}

// readTLSConfig builds the TLS settings for calls to vmanomaly.
// It returns nil when no TLS variables are set, keeping the system trust store.
// The returned caFile is VMANOMALY_TLS_CA_FILE, which the client re-reads after rotation;
// the client certificate files are re-read through GetClientCertificate.
func readTLSConfig() (cfg *tls.Config, caFile string, err error) {
	caFile = strings.TrimSpace(os.Getenv("VMANOMALY_TLS_CA_FILE"))
	caCert := strings.TrimSpace(os.Getenv("VMANOMALY_TLS_CA_CERT"))
	certFile := strings.TrimSpace(os.Getenv("VMANOMALY_TLS_CERT_FILE"))
	keyFile := strings.TrimSpace(os.Getenv("VMANOMALY_TLS_KEY_FILE"))
	insecureSkipVerify := false
	if v := strings.TrimSpace(os.Getenv("VMANOMALY_TLS_INSECURE_SKIP_VERIFY")); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return nil, "", fmt.Errorf("failed to parse VMANOMALY_TLS_INSECURE_SKIP_VERIFY: %w", err)
		}
		insecureSkipVerify = parsed
	}
	if caFile == "" && caCert == "" && certFile == "" && keyFile == "" && !insecureSkipVerify {
		return nil, "", nil
	}
	if caFile != "" && caCert != "" {
		return nil, "", fmt.Errorf("VMANOMALY_TLS_CA_FILE and VMANOMALY_TLS_CA_CERT are mutually exclusive")
	}
	if insecureSkipVerify && (caFile != "" || caCert != "") {
		// InsecureSkipVerify silently ignores RootCAs
		return nil, "", fmt.Errorf("VMANOMALY_TLS_INSECURE_SKIP_VERIFY cannot be combined with a custom CA")
	}
	if (certFile == "") != (keyFile == "") {
		return nil, "", fmt.Errorf("VMANOMALY_TLS_CERT_FILE and VMANOMALY_TLS_KEY_FILE must be set together")
	}

	cfg = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: insecureSkipVerify, //nolint:gosec // opt-in via VMANOMALY_TLS_INSECURE_SKIP_VERIFY
	}
	caPEM, caSource := []byte(caCert), "VMANOMALY_TLS_CA_CERT"
	if caFile != "" {
		data, err := os.ReadFile(caFile)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read VMANOMALY_TLS_CA_FILE: %w", err)
		}
		caPEM, caSource = data, "VMANOMALY_TLS_CA_FILE"
	}
	if caFile != "" || caCert != "" {
		pool, err := tlsfiles.CertPool(caPEM, caSource)
		if err != nil {
			return nil, "", err
		}
		cfg.RootCAs = pool
	}
	if certFile != "" {
		reloader, err := tlsfiles.NewCertReloader(certFile, keyFile)
		if err != nil {
			return nil, "", fmt.Errorf("failed to load VMANOMALY_TLS_CERT_FILE and VMANOMALY_TLS_KEY_FILE: %w", err)
		}
		cfg.GetClientCertificate = reloader.GetClientCertificate
	}
	return cfg, caFile, nil
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

// Stateless reports whether the Streamable HTTP server ignores Mcp-Session-Id
// values instead of validating them. It applies only in http mode.
func (c *Config) Stateless() bool {
	return c.stateless
}

// DisableLocalhostProtection reports whether the DNS-rebinding guard is off.
// The guard rejects a loopback request whose Host header is not a localhost value.
// It applies in sse and http modes.
func (c *Config) DisableLocalhostProtection() bool {
	return c.disableLocalhostProtection
}

func (c *Config) HeartbeatInterval() time.Duration {
	return c.heartbeatInterval
}

func (c *Config) LogLevel() string {
	return c.logLevel
}

// LogFormat returns the log output format: "text" or "json".
func (c *Config) LogFormat() string {
	return c.logFormat
}

// InstructionsOverride returns text that replaces the default MCP server instructions.
// Empty means the default instructions are used.
func (c *Config) InstructionsOverride() string {
	return c.instructionsOverride
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

// PassthroughHeaders returns the canonical names of headers forwarded from each
// incoming MCP request to vmanomaly. They override VMANOMALY_HEADERS and the
// VMANOMALY_BEARER_TOKEN Authorization header on collision.
func (c *Config) PassthroughHeaders() []string {
	return c.passthroughHeaders
}

// AuthToken returns the bearer token MCP clients must present in sse and http modes.
// Empty means the MCP endpoints are not authenticated.
func (c *Config) AuthToken() string {
	return c.authToken
}

func (c *Config) RequestTimeout() time.Duration {
	return c.requestTimeout
}

// TLSConfig returns TLS settings for calls to vmanomaly, or nil to use the defaults.
// A client certificate from files is served through GetClientCertificate and
// reloaded after rotation.
func (c *Config) TLSConfig() *tls.Config {
	return c.tlsConfig
}

// TLSCAFile returns VMANOMALY_TLS_CA_FILE, which the vmanomaly client re-reads
// after rotation, or "" when the CA is unset or given inline.
func (c *Config) TLSCAFile() string {
	return c.tlsCAFile
}
