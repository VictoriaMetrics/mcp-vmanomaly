package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/VictoriaMetrics-Community/mcp-vmanomaly/internal/vmanomaly"
)

func TestListModels_Error(t *testing.T) {
	mock := &MockClient{
		ListModelsFunc: func(ctx context.Context) (*vmanomaly.ModelsListResponse, error) {
			return nil, errors.New("API error")
		},
	}

	_, err := mock.ListModels(context.Background())

	if err == nil {
		t.Error("expected error from API")
	}
}

func TestGetModelSchema_Error(t *testing.T) {
	mock := &MockClient{
		GetModelSchemaFunc: func(ctx context.Context, modelClass string) (map[string]any, error) {
			return nil, errors.New("model not found")
		},
	}

	_, err := mock.GetModelSchema(context.Background(), "invalid")

	if err == nil {
		t.Error("expected error from API")
	}
}

func TestValidateModel_InvalidModel(t *testing.T) {
	mock := &MockClient{
		ValidateModelFunc: func(ctx context.Context, modelSpec map[string]any) (*vmanomaly.ModelValidationResponse, error) {
			return &vmanomaly.ModelValidationResponse{
				Valid:     false,
				ModelSpec: map[string]any{"error": "missing required field: class"},
			}, nil
		},
	}

	result, err := mock.ValidateModel(context.Background(), map[string]any{"invalid": "config"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid model")
	}
}

func TestGenerateConfig_Defaults(t *testing.T) {
	var receivedReq *vmanomaly.ConfigGenerationRequest

	mock := &MockClient{
		GenerateConfigFunc: func(ctx context.Context, req *vmanomaly.ConfigGenerationRequest) (string, error) {
			receivedReq = req
			return "schedulers:\n  fit_window: 1d", nil
		},
	}

	configReq := &vmanomaly.ConfigGenerationRequest{
		Query:         "up",
		Step:          "1m",
		DatasourceURL: "http://vm:8428",
		ModelSpec:     map[string]any{"class": "zscore"},
		FitWindow:     "1d",
		FitEvery:      "1d",
	}

	_, err := mock.GenerateConfig(context.Background(), configReq)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedReq.FitWindow != "1d" {
		t.Errorf("expected default fit_window=1d, got %s", receivedReq.FitWindow)
	}

	if receivedReq.FitEvery != "1d" {
		t.Errorf("expected default fit_every=1d, got %s", receivedReq.FitEvery)
	}
}

func TestGenerateConfig_OverrideDefaults(t *testing.T) {
	var receivedReq *vmanomaly.ConfigGenerationRequest

	mock := &MockClient{
		GenerateConfigFunc: func(ctx context.Context, req *vmanomaly.ConfigGenerationRequest) (string, error) {
			receivedReq = req
			return "config", nil
		},
	}

	tenantID := "tenant1"
	configReq := &vmanomaly.ConfigGenerationRequest{
		Query:         "up",
		Step:          "1m",
		DatasourceURL: "http://vm:8428",
		ModelSpec:     map[string]any{"class": "zscore"},
		FitWindow:     "2h",
		FitEvery:      "30m",
		TenantID:      &tenantID,
	}

	_, err := mock.GenerateConfig(context.Background(), configReq)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedReq.FitWindow != "2h" {
		t.Errorf("expected fit_window=2h, got %s", receivedReq.FitWindow)
	}

	if receivedReq.FitEvery != "30m" {
		t.Errorf("expected fit_every=30m, got %s", receivedReq.FitEvery)
	}

	if receivedReq.TenantID == nil || *receivedReq.TenantID != "tenant1" {
		t.Error("expected tenant_id=tenant1")
	}
}
