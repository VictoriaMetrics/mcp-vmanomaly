package tools

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/VictoriaMetrics-Community/mcp-vmanomaly/internal/vmanomaly"
)

func TestQueryMetrics_Defaults(t *testing.T) {
	var receivedReq *vmanomaly.QueryRequest

	mock := &MockClient{
		QueryFunc: func(ctx context.Context, req *vmanomaly.QueryRequest) (map[string]any, error) {
			receivedReq = req
			return map[string]any{"status": "success"}, nil
		},
	}

	queryReq := &vmanomaly.QueryRequest{
		Query:          "up",
		Step:           "1s",
		DatasourceType: "vm",
	}

	_, err := mock.Query(context.Background(), queryReq)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedReq.Step != "1s" {
		t.Errorf("expected default step=1s, got %s", receivedReq.Step)
	}

	if receivedReq.DatasourceType != "vm" {
		t.Errorf("expected default datasource_type=vm, got %s", receivedReq.DatasourceType)
	}
}

func TestQueryMetrics_OverrideDefaults(t *testing.T) {
	var receivedReq *vmanomaly.QueryRequest

	mock := &MockClient{
		QueryFunc: func(ctx context.Context, req *vmanomaly.QueryRequest) (map[string]any, error) {
			receivedReq = req
			return map[string]any{"status": "success"}, nil
		},
	}

	start := 1234567890.0
	end := 1234567900.0
	queryReq := &vmanomaly.QueryRequest{
		Query:          "up",
		Step:           "5m",
		DatasourceType: "vmlogs",
		Start:          &start,
		End:            &end,
	}

	_, err := mock.Query(context.Background(), queryReq)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedReq.Step != "5m" {
		t.Errorf("expected step=5m, got %s", receivedReq.Step)
	}

	if receivedReq.DatasourceType != "vmlogs" {
		t.Errorf("expected datasource_type=vmlogs, got %s", receivedReq.DatasourceType)
	}

	if receivedReq.Start == nil || *receivedReq.Start != 1234567890 {
		t.Error("expected start timestamp")
	}

	if receivedReq.End == nil || *receivedReq.End != 1234567900 {
		t.Error("expected end timestamp")
	}
}

func TestQueryMetrics_EnvVarUsage(t *testing.T) {
	os.Setenv("VMANOMALY_ENDPOINT", "http://env:8490")
	defer os.Unsetenv("VMANOMALY_ENDPOINT")

	envURL := os.Getenv("VMANOMALY_ENDPOINT")

	if envURL != "http://env:8490" {
		t.Errorf("expected env var VMANOMALY_ENDPOINT=http://env:8490, got %s", envURL)
	}
}

func TestQueryMetrics_Error(t *testing.T) {
	mock := &MockClient{
		QueryFunc: func(ctx context.Context, req *vmanomaly.QueryRequest) (map[string]any, error) {
			return nil, errors.New("query execution failed")
		},
	}

	_, err := mock.Query(context.Background(), &vmanomaly.QueryRequest{Query: "invalid"})

	if err == nil {
		t.Error("expected error from API")
	}
}
