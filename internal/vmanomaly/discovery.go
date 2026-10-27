package vmanomaly

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
)

// Discovery API scope limits. Window length is elapsed Unix seconds; selector
// lengths are UTF-8 bytes. Zero result limit leaves the default to the server.
const (
	maxDiscoverySelectors     = 10
	maxDiscoveryWindowSeconds = 31 * 24 * 60 * 60
	maxDiscoveryResults       = 1000
	// Seconds; zero is omitted on the wire to use the server default.
	maxDiscoveryTimeoutSeconds = 30
	maxDiscoverySelectorBytes  = 4096

	// Conservative client memory budgets, not bounds on upstream TSDB scan work.
	maxDiscoveryRequestBytes  = 64 * 1024
	maxDiscoveryResponseBytes = 3 * 1024 * 1024
)

// DiscoveryRequest scopes one metadata request; it does not query historical samples.
type DiscoveryRequest struct {
	Operation       string   `json:"operation"`
	DatasourceURL   string   `json:"datasource_url"`
	TenantID        string   `json:"tenant_id,omitempty"`
	Match           []string `json:"match"`
	Start           float64  `json:"start"`
	End             float64  `json:"end"`
	Label           string   `json:"label,omitempty"`
	Limit           int      `json:"limit,omitempty"`
	TimeoutSeconds  float64  `json:"timeout_seconds,omitempty"`
	PassAuthHeaders bool     `json:"pass_auth_headers,omitempty"`
}

func (c *Client) Discover(ctx context.Context, request DiscoveryRequest) (json.RawMessage, error) {
	switch request.Operation {
	case "metric_names", "label_names", "label_values", "series":
	default:
		return nil, fmt.Errorf("operation must be metric_names, label_names, label_values or series")
	}
	if len(request.Match) == 0 {
		return nil, fmt.Errorf("match must contain at least one selector")
	}
	if len(request.Match) > maxDiscoverySelectors {
		return nil, fmt.Errorf("match contains %d selectors; maximum is %d", len(request.Match), maxDiscoverySelectors)
	}
	if request.End <= request.Start {
		return nil, fmt.Errorf("end must be greater than start (Unix seconds)")
	}
	if request.End-request.Start > maxDiscoveryWindowSeconds {
		return nil, fmt.Errorf("discovery window exceeds %d seconds; shorten start/end", maxDiscoveryWindowSeconds)
	}
	if request.Limit < 0 || request.Limit > maxDiscoveryResults {
		return nil, fmt.Errorf("limit must be between 1 and %d, or 0 to use the server default; got %d", maxDiscoveryResults, request.Limit)
	}
	if math.IsNaN(request.TimeoutSeconds) || math.IsInf(request.TimeoutSeconds, 0) || request.TimeoutSeconds < 0 || request.TimeoutSeconds > maxDiscoveryTimeoutSeconds {
		return nil, fmt.Errorf("timeout_seconds must be finite and between 0 and %d; 0 uses the server default", maxDiscoveryTimeoutSeconds)
	}
	if request.Operation == "label_values" && strings.TrimSpace(request.Label) == "" {
		return nil, fmt.Errorf("label_values requires a nonblank label")
	}
	for i, selector := range request.Match {
		if strings.TrimSpace(selector) == "" {
			return nil, fmt.Errorf("match[%d] must not be blank", i)
		}
		if len(selector) > maxDiscoverySelectorBytes {
			return nil, fmt.Errorf("match[%d] exceeds %d bytes (%d bytes)", i, maxDiscoverySelectorBytes, len(selector))
		}
	}
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("cannot encode discovery request as JSON: %w", err)
	}
	if len(body) > maxDiscoveryRequestBytes {
		return nil, fmt.Errorf("discovery request exceeds %d bytes (%d bytes)", maxDiscoveryRequestBytes, len(body))
	}
	data, err := c.doRequestLimited(ctx, http.MethodPost, "/api/v1/discovery", json.RawMessage(body), maxDiscoveryResponseBytes)
	if err != nil {
		return nil, err
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("discovery returned invalid JSON")
	}
	return data, nil
}
