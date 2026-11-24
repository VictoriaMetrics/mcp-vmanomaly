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
	// Tools that call vmanomaly or a datasource are open-world; only embedded docs and guides are not.
	remoteReadOnly := hints{readOnly: true, openWorld: true}
	embeddedReadOnly := hints{readOnly: true}
	want := map[string]hints{
		"vmanomaly_health_check":                   remoteReadOnly,
		"vmanomaly_get_buildinfo":                  remoteReadOnly,
		"vmanomaly_get_server_queries":             remoteReadOnly,
		"vmanomaly_get_metrics":                    remoteReadOnly,
		"vmanomaly_list_models":                    remoteReadOnly,
		"vmanomaly_get_server_models":              remoteReadOnly,
		"vmanomaly_get_model_schema":               remoteReadOnly,
		"vmanomaly_validate_model_config":          remoteReadOnly,
		"vmanomaly_validate_config":                remoteReadOnly,
		"vmanomaly_generate_preset_artifacts":      remoteReadOnly,
		"vmanomaly_get_preset_guide":               embeddedReadOnly,
		"vmanomaly_estimate_deployment_resources":  remoteReadOnly,
		"vmanomaly_estimate_inference_capacity":    remoteReadOnly,
		"vmanomaly_get_deployment_sizing_profiles": remoteReadOnly,
		"vmanomaly_discover_metrics":               remoteReadOnly,
		"vmanomaly_check_compatibility":            remoteReadOnly,
		"vmanomaly_generate_alert_rule":            remoteReadOnly,
		"vmanomaly_timeseries_characteristics":     remoteReadOnly,
		"vmanomaly_create_autotune_task":           {openWorld: true},
		"vmanomaly_get_autotune_task":              remoteReadOnly,
		"vmanomaly_cancel_autotune_task":           {destructive: true, openWorld: true},
		"vmanomaly_search_docs":                    embeddedReadOnly,
		"vmanomaly_read_doc_section":               embeddedReadOnly,
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
