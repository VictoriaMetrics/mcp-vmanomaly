package vmanomaly

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// DiscoveryRequest scopes one metadata request; it does not query historical samples.
type DiscoveryRequest struct {
	Operation       string   `json:"operation" jsonschema:"enum=metric_names,enum=label_names,enum=label_values,enum=series"`
	DatasourceURL   string   `json:"datasource_url"`
	TenantID        string   `json:"tenant_id,omitempty"`
	Match           []string `json:"match" jsonschema:"minItems=1,maxItems=10"`
	Start           float64  `json:"start"`
	End             float64  `json:"end"`
	Label           string   `json:"label,omitempty"`
	Limit           int      `json:"limit,omitempty" jsonschema:"minimum=1,maximum=1000"`
	TimeoutSeconds  float64  `json:"timeout_seconds,omitempty"`
	PassAuthHeaders bool     `json:"pass_auth_headers,omitempty"`
}

func (c *Client) Discover(ctx context.Context, request DiscoveryRequest) (json.RawMessage, error) {
	switch request.Operation {
	case "metric_names", "label_names", "label_values", "series":
	default:
		return nil, fmt.Errorf("unknown discovery operation")
	}
	if len(request.Match) < 1 || len(request.Match) > 10 || request.End <= request.Start || request.End-request.Start > 31*86400 || request.Limit < 0 || request.Limit > 1000 {
		return nil, fmt.Errorf("invalid discovery scope or budget")
	}
	for _, selector := range request.Match {
		if strings.TrimSpace(selector) == "" || len(selector) > 4096 {
			return nil, fmt.Errorf("invalid discovery selector")
		}
	}
	body, err := json.Marshal(request)
	if err != nil || len(body) > 65536 {
		return nil, fmt.Errorf("discovery request exceeds JSON budget or contains invalid values")
	}
	data, err := c.doRequestLimited(ctx, http.MethodPost, "/api/v1/discovery", json.RawMessage(body), 3*1024*1024)
	if err != nil {
		return nil, err
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("discovery returned invalid JSON")
	}
	return data, nil
}
