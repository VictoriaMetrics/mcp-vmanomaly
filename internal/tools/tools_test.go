package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/mark3labs/mcp-go/server"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/vmanomaly"
)

func TestHealthCheck_Error(t *testing.T) {
	mock := &MockClient{
		GetHealthFunc: func(ctx context.Context) (map[string]any, error) {
			return nil, errors.New("connection refused")
		},
	}

	_, err := mock.GetHealth(context.Background())

	if err == nil {
		t.Error("expected error from API")
	}
}

func TestEveryToolHasTitleAndExpectedHints(t *testing.T) {
	type hints struct{ readOnly, destructive, openWorld bool }
	readOnly := hints{readOnly: true}
	want := map[string]hints{
		"vmanomaly_health_check":                   readOnly,
		"vmanomaly_get_buildinfo":                  readOnly,
		"vmanomaly_get_server_queries":             readOnly,
		"vmanomaly_get_metrics":                    readOnly,
		"vmanomaly_list_models":                    readOnly,
		"vmanomaly_get_server_models":              readOnly,
		"vmanomaly_get_model_schema":               readOnly,
		"vmanomaly_validate_model_config":          readOnly,
		"vmanomaly_validate_config":                readOnly,
		"vmanomaly_generate_preset_artifacts":      readOnly,
		"vmanomaly_get_preset_guide":               readOnly,
		"vmanomaly_estimate_deployment_resources":  readOnly,
		"vmanomaly_estimate_inference_capacity":    readOnly,
		"vmanomaly_get_deployment_sizing_profiles": readOnly,
		"vmanomaly_discover_metrics":               {readOnly: true, openWorld: true},
		"vmanomaly_check_compatibility":            readOnly,
		"vmanomaly_generate_alert_rule":            readOnly,
		"vmanomaly_timeseries_characteristics":     readOnly,
		"vmanomaly_create_autotune_task":           {},
		"vmanomaly_get_autotune_task":              readOnly,
		"vmanomaly_cancel_autotune_task":           {destructive: true},
		"vmanomaly_search_docs":                    readOnly,
		"vmanomaly_read_doc_section":               readOnly,
	}

	s := server.NewMCPServer("test", "1")
	RegisterTools(s, vmanomaly.NewClient("http://127.0.0.1:1", "", nil))
	registered := s.ListTools()
	for name := range want {
		if registered[name] == nil {
			t.Errorf("%s is expected but not registered", name)
		}
	}
	for name, tool := range registered {
		expected, ok := want[name]
		if !ok {
			t.Errorf("%s has no expected hints; add it to this table", name)
			continue
		}
		a := tool.Tool.Annotations
		if a.Title == "" {
			t.Errorf("%s has no annotation title", name)
		}
		if a.ReadOnlyHint == nil || a.DestructiveHint == nil || a.OpenWorldHint == nil {
			t.Errorf("%s does not set readOnlyHint, destructiveHint and openWorldHint explicitly", name)
			continue
		}
		got := hints{readOnly: *a.ReadOnlyHint, destructive: *a.DestructiveHint, openWorld: *a.OpenWorldHint}
		if got != expected {
			t.Errorf("%s hints = %+v, want %+v", name, got, expected)
		}
	}
}
