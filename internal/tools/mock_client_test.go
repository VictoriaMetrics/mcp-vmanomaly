package tools

import (
	"context"
	"errors"

	"github.com/VictoriaMetrics-Community/mcp-vmanomaly/internal/vmanomaly"
)

// MockClient is a mock implementation of vmanomaly.Client for testing
type MockClient struct {
	GetHealthFunc           func(ctx context.Context) (map[string]any, error)
	GetBuildInfoFunc        func(ctx context.Context) (map[string]any, error)
	ListModelsFunc          func(ctx context.Context) (*vmanomaly.ModelsListResponse, error)
	GetModelSchemaFunc      func(ctx context.Context, modelClass string) (map[string]any, error)
	ValidateModelFunc       func(ctx context.Context, modelSpec map[string]any) (*vmanomaly.ModelValidationResponse, error)
	GenerateConfigFunc      func(ctx context.Context, req *vmanomaly.ConfigGenerationRequest) (string, error)
	CreateDetectionTaskFunc func(ctx context.Context, req *vmanomaly.AnomalyDetectionTaskRequest) (*vmanomaly.AnomalyDetectionTaskResponse, error)
	GetTaskStatusFunc       func(ctx context.Context, taskID string) (*vmanomaly.AnomalyDetectionTaskStatus, error)
	ListTasksFunc           func(ctx context.Context, limit int, status *string) (*vmanomaly.AnomalyDetectionTaskListResponse, error)
	CancelTaskFunc          func(ctx context.Context, taskID string) (map[string]bool, error)
	GetDetectionLimitsFunc  func(ctx context.Context) (*vmanomaly.AnomalyDetectionLimitsResponse, error)
	QueryFunc               func(ctx context.Context, req *vmanomaly.QueryRequest) (map[string]any, error)
}

func (m *MockClient) GetHealth(ctx context.Context) (map[string]any, error) {
	if m.GetHealthFunc != nil {
		return m.GetHealthFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *MockClient) GetBuildInfo(ctx context.Context) (map[string]any, error) {
	if m.GetBuildInfoFunc != nil {
		return m.GetBuildInfoFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *MockClient) ListModels(ctx context.Context) (*vmanomaly.ModelsListResponse, error) {
	if m.ListModelsFunc != nil {
		return m.ListModelsFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *MockClient) GetModelSchema(ctx context.Context, modelClass string) (map[string]any, error) {
	if m.GetModelSchemaFunc != nil {
		return m.GetModelSchemaFunc(ctx, modelClass)
	}
	return nil, errors.New("not implemented")
}

func (m *MockClient) ValidateModel(ctx context.Context, modelSpec map[string]any) (*vmanomaly.ModelValidationResponse, error) {
	if m.ValidateModelFunc != nil {
		return m.ValidateModelFunc(ctx, modelSpec)
	}
	return nil, errors.New("not implemented")
}

func (m *MockClient) GenerateConfig(ctx context.Context, req *vmanomaly.ConfigGenerationRequest) (string, error) {
	if m.GenerateConfigFunc != nil {
		return m.GenerateConfigFunc(ctx, req)
	}
	return "", errors.New("not implemented")
}

func (m *MockClient) CreateDetectionTask(ctx context.Context, req *vmanomaly.AnomalyDetectionTaskRequest) (*vmanomaly.AnomalyDetectionTaskResponse, error) {
	if m.CreateDetectionTaskFunc != nil {
		return m.CreateDetectionTaskFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *MockClient) GetTaskStatus(ctx context.Context, taskID string) (*vmanomaly.AnomalyDetectionTaskStatus, error) {
	if m.GetTaskStatusFunc != nil {
		return m.GetTaskStatusFunc(ctx, taskID)
	}
	return nil, errors.New("not implemented")
}

func (m *MockClient) ListTasks(ctx context.Context, limit int, status *string) (*vmanomaly.AnomalyDetectionTaskListResponse, error) {
	if m.ListTasksFunc != nil {
		return m.ListTasksFunc(ctx, limit, status)
	}
	return nil, errors.New("not implemented")
}

func (m *MockClient) CancelTask(ctx context.Context, taskID string) (map[string]bool, error) {
	if m.CancelTaskFunc != nil {
		return m.CancelTaskFunc(ctx, taskID)
	}
	return nil, errors.New("not implemented")
}

func (m *MockClient) GetDetectionLimits(ctx context.Context) (*vmanomaly.AnomalyDetectionLimitsResponse, error) {
	if m.GetDetectionLimitsFunc != nil {
		return m.GetDetectionLimitsFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *MockClient) Query(ctx context.Context, req *vmanomaly.QueryRequest) (map[string]any, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}
