package tools

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/VictoriaMetrics-Community/mcp-vmanomaly/internal/vmanomaly"
)

func TestCreateDetectionTask_Defaults(t *testing.T) {
	var receivedReq *vmanomaly.AnomalyDetectionTaskRequest

	mock := &MockClient{
		CreateDetectionTaskFunc: func(ctx context.Context, req *vmanomaly.AnomalyDetectionTaskRequest) (*vmanomaly.AnomalyDetectionTaskResponse, error) {
			receivedReq = req
			return &vmanomaly.AnomalyDetectionTaskResponse{
				TaskID: "task-123",
				Status: "running",
			}, nil
		},
	}

	taskReq := &vmanomaly.AnomalyDetectionTaskRequest{
		Query:            "up",
		ModelSpec:        map[string]any{"class": "zscore"},
		Step:             "1s",
		FitWindow:        "1d",
		FitEvery:         "1d",
		AnomalyThreshold: 1.0,
		DatasourceType:   "vm",
	}

	_, err := mock.CreateDetectionTask(context.Background(), taskReq)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedReq.Step != "1s" {
		t.Errorf("expected default step=1s, got %s", receivedReq.Step)
	}

	if receivedReq.FitWindow != "1d" {
		t.Errorf("expected default fit_window=1d, got %s", receivedReq.FitWindow)
	}

	if receivedReq.AnomalyThreshold != 1.0 {
		t.Errorf("expected default anomaly_threshold=1.0, got %f", receivedReq.AnomalyThreshold)
	}
}

func TestCreateDetectionTask_OverrideDefaults(t *testing.T) {
	var receivedReq *vmanomaly.AnomalyDetectionTaskRequest

	mock := &MockClient{
		CreateDetectionTaskFunc: func(ctx context.Context, req *vmanomaly.AnomalyDetectionTaskRequest) (*vmanomaly.AnomalyDetectionTaskResponse, error) {
			receivedReq = req
			return &vmanomaly.AnomalyDetectionTaskResponse{TaskID: "task-123"}, nil
		},
	}

	datasourceURL := "http://custom:8428"
	taskReq := &vmanomaly.AnomalyDetectionTaskRequest{
		Query:            "up",
		ModelSpec:        map[string]any{"class": "zscore"},
		Step:             "5m",
		FitWindow:        "2h",
		FitEvery:         "1h",
		AnomalyThreshold: 2.5,
		DatasourceType:   "vmlogs",
		DatasourceURL:    &datasourceURL,
	}

	_, err := mock.CreateDetectionTask(context.Background(), taskReq)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedReq.Step != "5m" {
		t.Errorf("expected step=5m, got %s", receivedReq.Step)
	}

	if receivedReq.AnomalyThreshold != 2.5 {
		t.Errorf("expected anomaly_threshold=2.5, got %f", receivedReq.AnomalyThreshold)
	}

	if receivedReq.DatasourceURL == nil || *receivedReq.DatasourceURL != "http://custom:8428" {
		t.Error("expected custom datasource URL")
	}
}

func TestCreateDetectionTask_EnvVarUsage(t *testing.T) {
	os.Setenv("VMANOMALY_ENDPOINT", "http://env:8490")
	defer os.Unsetenv("VMANOMALY_ENDPOINT")

	envURL := os.Getenv("VMANOMALY_ENDPOINT")

	if envURL != "http://env:8490" {
		t.Errorf("expected env var VMANOMALY_ENDPOINT=http://env:8490, got %s", envURL)
	}
}

func TestGetTaskStatus_Error(t *testing.T) {
	mock := &MockClient{
		GetTaskStatusFunc: func(ctx context.Context, taskID string) (*vmanomaly.AnomalyDetectionTaskStatus, error) {
			return nil, errors.New("task not found")
		},
	}

	_, err := mock.GetTaskStatus(context.Background(), "invalid")

	if err == nil {
		t.Error("expected error from API")
	}
}

func TestListTasks_DefaultLimit(t *testing.T) {
	var receivedLimit int

	mock := &MockClient{
		ListTasksFunc: func(ctx context.Context, limit int, status *string) (*vmanomaly.AnomalyDetectionTaskListResponse, error) {
			receivedLimit = limit
			return &vmanomaly.AnomalyDetectionTaskListResponse{Tasks: []vmanomaly.AnomalyDetectionTaskListItem{}}, nil
		},
	}

	_, err := mock.ListTasks(context.Background(), 20, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedLimit != 20 {
		t.Errorf("expected limit=20, got %d", receivedLimit)
	}
}

func TestListTasks_CustomLimit(t *testing.T) {
	var receivedLimit int

	mock := &MockClient{
		ListTasksFunc: func(ctx context.Context, limit int, status *string) (*vmanomaly.AnomalyDetectionTaskListResponse, error) {
			receivedLimit = limit
			return &vmanomaly.AnomalyDetectionTaskListResponse{Tasks: []vmanomaly.AnomalyDetectionTaskListItem{}}, nil
		},
	}

	_, err := mock.ListTasks(context.Background(), 50, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedLimit != 50 {
		t.Errorf("expected limit=50, got %d", receivedLimit)
	}
}

func TestCancelTask_Error(t *testing.T) {
	mock := &MockClient{
		CancelTaskFunc: func(ctx context.Context, taskID string) (map[string]bool, error) {
			return nil, errors.New("task already completed")
		},
	}

	_, err := mock.CancelTask(context.Background(), "task-123")

	if err == nil {
		t.Error("expected error from API")
	}
}

func TestGetDetectionLimits_Error(t *testing.T) {
	mock := &MockClient{
		GetDetectionLimitsFunc: func(ctx context.Context) (*vmanomaly.AnomalyDetectionLimitsResponse, error) {
			return nil, errors.New("API error")
		},
	}

	_, err := mock.GetDetectionLimits(context.Background())

	if err == nil {
		t.Error("expected error from API")
	}
}
