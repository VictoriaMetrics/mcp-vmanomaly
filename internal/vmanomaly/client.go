package vmanomaly

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client represents a vmanomaly API client
type Client struct {
	baseURL       string
	httpClient    *http.Client
	bearerToken   string
	customHeaders map[string]string
}

func NewClient(baseURL, bearerToken string, customHeaders map[string]string) *Client {
	return &Client{
		baseURL:       baseURL,
		bearerToken:   bearerToken,
		customHeaders: customHeaders,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) doRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	url := fmt.Sprintf("%s%s", c.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.bearerToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.bearerToken))
	}

	for key, value := range c.customHeaders {
		req.Header.Set(key, value)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (c *Client) GetHealth(ctx context.Context) (map[string]any, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// ============================================================================
// Model Configuration Methods
// ============================================================================

// ListModels returns a list of available anomaly detection model types
func (c *Client) ListModels(ctx context.Context) (*ModelsListResponse, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, "/api/v1/models", nil)
	if err != nil {
		return nil, err
	}

	var result ModelsListResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// GetServerModels returns configured runtime models from the vmanomaly server
func (c *Client) GetServerModels(ctx context.Context) (*ServerModelsResponse, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, "/api/v1/server/models", nil)
	if err != nil {
		return nil, err
	}

	var result ServerModelsResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

func (c *Client) GetModelSchema(ctx context.Context, modelClass string) (map[string]any, error) {
	path := fmt.Sprintf("/api/v1/model/schema?model_class=%s", modelClass)
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

func (c *Client) ValidateModel(ctx context.Context, modelSpec map[string]any) (*ModelValidationResponse, error) {
	respBody, err := c.doRequest(ctx, http.MethodPost, "/api/v1/model/validate", modelSpec)
	if err != nil {
		return nil, err
	}

	var result ModelValidationResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

func (c *Client) GenerateConfig(ctx context.Context, req *ConfigGenerationRequest) (string, error) {
	// Build query parameters
	path := fmt.Sprintf("/api/vmanomaly/config.yaml?step=%s&query=%s&datasource_url=%s&fit_window=%s&fit_every=%s",
		req.Step, req.Query, req.DatasourceURL, req.FitWindow, req.FitEvery)

	if req.TenantID != nil {
		path += fmt.Sprintf("&tenant_id=%s", *req.TenantID)
	}
	if req.InferEvery != nil {
		path += fmt.Sprintf("&infer_every=%s", *req.InferEvery)
	}

	modelSpecJSON, err := json.Marshal(req.ModelSpec)
	if err != nil {
		return "", fmt.Errorf("failed to encode model_spec: %w", err)
	}
	path += fmt.Sprintf("&model_spec=%s", string(modelSpecJSON))

	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}

	return string(respBody), nil
}

// ============================================================================
// Anomaly Detection Task Methods
// ============================================================================

func (c *Client) CreateDetectionTask(ctx context.Context, req *AnomalyDetectionTaskRequest) (*AnomalyDetectionTaskResponse, error) {
	respBody, err := c.doRequest(ctx, http.MethodPost, "/api/v1/anomaly_detection/tasks", req)
	if err != nil {
		return nil, err
	}

	var result AnomalyDetectionTaskResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

func (c *Client) GetTaskStatus(ctx context.Context, taskID string) (*AnomalyDetectionTaskStatus, error) {
	path := fmt.Sprintf("/api/v1/anomaly_detection/tasks/%s", taskID)
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var result AnomalyDetectionTaskStatus
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

func (c *Client) ListTasks(ctx context.Context, limit int, status *string) (*AnomalyDetectionTaskListResponse, error) {
	path := fmt.Sprintf("/api/v1/anomaly_detection/tasks?limit=%d", limit)
	if status != nil {
		path += fmt.Sprintf("&status=%s", *status)
	}

	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var result AnomalyDetectionTaskListResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

func (c *Client) CancelTask(ctx context.Context, taskID string) (map[string]bool, error) {
	path := fmt.Sprintf("/api/v1/anomaly_detection/tasks/%s", taskID)
	respBody, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return nil, err
	}

	var result map[string]bool
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

func (c *Client) GetDetectionLimits(ctx context.Context) (*AnomalyDetectionLimitsResponse, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, "/api/v1/anomaly_detection/limits", nil)
	if err != nil {
		return nil, err
	}

	var result AnomalyDetectionLimitsResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// Query Methods
// ============================================================================

// Query executes a PromQL query against the datasource
func (c *Client) Query(ctx context.Context, req *QueryRequest) (map[string]any, error) {
	respBody, err := c.doRequest(ctx, http.MethodPost, "/api/v1/query", req)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

func (c *Client) GetBuildInfo(ctx context.Context) (map[string]any, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, "/api/v1/server/buildinfo", nil)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

func (c *Client) GetServerQueries(ctx context.Context) (ServerQueriesResponse, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, "/api/v1/server/queries", nil)
	if err != nil {
		return nil, err
	}

	var result ServerQueriesResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

func (c *Client) ValidateConfig(ctx context.Context, config map[string]any) (*OkValidationResponse, error) {
	respBody, err := c.doRequest(ctx, http.MethodPost, "/api/v1/config/validate", config)
	if err != nil {
		return nil, err
	}

	var result OkValidationResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

func (c *Client) Metrics(ctx context.Context, config map[string]any) (string, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, "/metrics", config)
	if err != nil {
		return "", err
	}

	return string(respBody), nil
}

// ============================================================================
// Compatibility Check Methods
// ============================================================================

// Compatibility checks if persisted state is compatible with the runtime version
func (c *Client) Compatibility(ctx context.Context, versionTo *string) (*CompatibilityCheckResponse, error) {
	path := "/api/v1/compatibility"
	if versionTo != nil {
		path += fmt.Sprintf("?version_to=%s", *versionTo)
	}

	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var result CompatibilityCheckResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse compatibility response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// Alert Rule Methods
// ============================================================================

// GenerateAlertRule generates a VMAlert rule configuration for anomaly detection
func (c *Client) GenerateAlertRule(ctx context.Context, req *AlertRuleRequest) (string, error) {
	// Build query parameters
	path := fmt.Sprintf("/api/vmanomaly/example-alert-rule.yaml?step=%s&query=%s",
		url.QueryEscape(req.Step), url.QueryEscape(req.Query))

	if req.AnomalyThreshold != nil {
		path += fmt.Sprintf("&anomaly_threshold=%g", *req.AnomalyThreshold)
	}
	if req.RuleName != nil {
		path += fmt.Sprintf("&rule_name=%s", url.QueryEscape(*req.RuleName))
	}
	if req.GroupName != nil {
		path += fmt.Sprintf("&group_name=%s", url.QueryEscape(*req.GroupName))
	}
	if req.RuleDescription != nil {
		path += fmt.Sprintf("&rule_description=%s", url.QueryEscape(*req.RuleDescription))
	}
	if req.InferEvery != nil {
		path += fmt.Sprintf("&infer_every=%s", url.QueryEscape(*req.InferEvery))
	}

	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}

	return string(respBody), nil
}
