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
	Deployment     *DeploymentOptions   `json:"deployment,omitempty"`
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
	for _, field := range request.RequiredInputs {
		if field != "writer.datasource_url" && field != "writer.tenant_id" {
			return nil, fmt.Errorf("unsupported required input: %q", field)
		}
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

// DeploymentOptions selects a bounded server-owned adapter; no local rendering or deployment occurs.
type DeploymentOptions struct {
	Targets         []string             `json:"targets"`
	Storage         string               `json:"storage"`
	Image           string               `json:"image"`
	Name            string               `json:"name,omitempty"`
	Namespace       string               `json:"namespace,omitempty"`
	ChartVersion    string               `json:"chart_version,omitempty"`
	OperatorVersion string               `json:"operator_version,omitempty"`
	LicenseSecret   string               `json:"license_secret,omitempty"`
	VolumeSize      string               `json:"volume_size,omitempty"`
	StorageClass    string               `json:"storage_class,omitempty"`
	Resources       *DeploymentResources `json:"resources,omitempty"`
}

type DeploymentResources struct {
	Requests *ResourceValues `json:"requests,omitempty"`
	Limits   *ResourceValues `json:"limits,omitempty"`
}

type ResourceValues struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}
