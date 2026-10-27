package vmanomaly

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Conservative client memory budgets for complete configurations and generated
// bundles. These byte limits do not establish deployment or resource readiness.
const (
	maxArtifactRequestBytes  = 1024 * 1024
	maxArtifactResponseBytes = 8 * 1024 * 1024
)

// ArtifactRequest carries a complete runtime configuration; generation does not deploy it.
type ArtifactRequest struct {
	RequiredInputs []string             `json:"required_inputs,omitempty"`
	Config         map[string]any       `json:"config"`
	Alerts         *ArtifactAlertPolicy `json:"alerts,omitempty"`
}

type ArtifactAlertPolicy struct {
	EntityLabels       []string `json:"entity_labels,omitempty"`
	Threshold          *float64 `json:"threshold,omitempty"`
	EvaluationSeconds  *int     `json:"evaluation_seconds,omitempty"`
	PersistenceSeconds *int     `json:"persistence_seconds,omitempty"`
	IncidentBaseURL    string   `json:"incident_base_url,omitempty"`
}

func (c *Client) GenerateArtifacts(ctx context.Context, request ArtifactRequest) (json.RawMessage, error) {
	if request.Config == nil {
		return nil, fmt.Errorf("config is required")
	}
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("artifact request is not JSON serializable")
	}
	// Bound the request as well as the result; the backend owns configuration validation.
	if len(body) > maxArtifactRequestBytes {
		return nil, fmt.Errorf("artifact request exceeds 1 MiB")
	}
	data, err := c.doRequestLimited(ctx, http.MethodPost, "/api/v1/preset/artifacts", json.RawMessage(body), maxArtifactResponseBytes)
	if err != nil {
		return nil, err
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("artifact server returned invalid JSON")
	}
	return data, nil
}
