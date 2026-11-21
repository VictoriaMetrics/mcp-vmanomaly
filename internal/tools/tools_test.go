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

func TestEveryToolHasTitleAndExplicitHints(t *testing.T) {
	s := server.NewMCPServer("test", "1")
	RegisterTools(s, vmanomaly.NewClient("http://127.0.0.1:1", "", nil))
	registered := s.ListTools()
	if len(registered) == 0 {
		t.Fatal("no tools registered")
	}

	writeTools := map[string]bool{
		"vmanomaly_create_autotune_task": true,
		"vmanomaly_cancel_autotune_task": true,
	}
	for name, tool := range registered {
		a := tool.Tool.Annotations
		if a.Title == "" {
			t.Errorf("%s has no annotation title", name)
		}
		if a.ReadOnlyHint == nil || a.DestructiveHint == nil || a.OpenWorldHint == nil {
			t.Errorf("%s does not set readOnlyHint, destructiveHint and openWorldHint explicitly", name)
			continue
		}
		if readOnly := *a.ReadOnlyHint; readOnly == writeTools[name] {
			t.Errorf("%s readOnlyHint = %t, want %t", name, readOnly, !writeTools[name])
		}
		if *a.ReadOnlyHint && *a.DestructiveHint {
			t.Errorf("%s is read-only but marked destructive", name)
		}
	}
}
