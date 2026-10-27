package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/vmanomaly"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestArtifactToolPreservesStructuredResultAndErrors(t *testing.T) {
	var status atomic.Int32
	status.Store(http.StatusOK)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(int(status.Load()))
		if _, err := w.Write([]byte(`{"schema_version":1,"validation":"schema-valid","warnings":["not live-tested"]}`)); err != nil {
			t.Error(err)
		}
	}))
	defer upstream.Close()
	s := server.NewMCPServer("test", "1")
	RegisterArtifactTools(s, vmanomaly.NewClient(upstream.URL, "", nil))
	tool := s.GetTool("vmanomaly_generate_preset_artifacts")
	if tool == nil || tool.Tool.Annotations.ReadOnlyHint == nil || !*tool.Tool.Annotations.ReadOnlyHint {
		t.Fatal("missing read-only tool")
	}
	var request mcp.CallToolRequest
	request.Params.Arguments = map[string]any{"config": map[string]any{"models": map[string]any{}}}
	result, err := tool.Handler(context.Background(), request)
	if err != nil || result.IsError {
		t.Fatalf("%v %v", result, err)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil || string(encoded) != `{"schema_version":1,"validation":"schema-valid","warnings":["not live-tested"]}` {
		t.Fatalf("structured response changed: %s %v", encoded, err)
	}
	status.Store(http.StatusNotFound)
	result, err = tool.Handler(context.Background(), request)
	if err != nil || !result.IsError {
		t.Fatal("unsupported endpoint appeared successful")
	}
}

func TestArtifactArgsPreserveOmittedAndExplicitValues(t *testing.T) {
	for _, alerts := range []map[string]any{
		nil,
		{},
		{"threshold": 1.5, "evaluation_seconds": float64(60), "persistence_seconds": float64(0), "entity_labels": []any{"instance"}, "incident_base_url": "https://anomaly.example/prefix"},
	} {
		t.Run(fmt.Sprint(alerts), func(t *testing.T) {
			config := map[string]any{"models": map[string]any{}}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var got map[string]any
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Error(err)
				}
				expected := map[string]any{"config": config, "required_inputs": []any{"writer.tenant_id"}}
				if alerts != nil {
					expected["alerts"] = alerts
				}
				if !reflect.DeepEqual(got, expected) {
					t.Errorf("request changed: got %#v; want %#v", got, expected)
				}
				if _, err := w.Write([]byte(`{"validation":"schema-valid"}`)); err != nil {
					t.Error(err)
				}
			}))
			defer upstream.Close()
			s := server.NewMCPServer("test", "1")
			RegisterArtifactTools(s, vmanomaly.NewClient(upstream.URL, "", nil))
			var request mcp.CallToolRequest
			request.Params.Arguments = map[string]any{"config": config, "required_inputs": []string{"writer.tenant_id"}}
			if alerts != nil {
				request.Params.Arguments.(map[string]any)["alerts"] = alerts
			}
			result, err := s.GetTool("vmanomaly_generate_preset_artifacts").Handler(context.Background(), request)
			if err != nil || result.IsError {
				t.Fatalf("%v %v", result, err)
			}
		})
	}
}

func TestArtifactSchemaDescribesInputs(t *testing.T) {
	s := server.NewMCPServer("test", "1")
	RegisterArtifactTools(s, vmanomaly.NewClient("http://unused", "", nil))
	data, err := json.Marshal(s.GetTool("vmanomaly_generate_preset_artifacts").Tool)
	if err != nil {
		t.Fatal(err)
	}
	var tool map[string]any
	if err := json.Unmarshal(data, &tool); err != nil {
		t.Fatal(err)
	}
	schema := tool["inputSchema"].(map[string]any)
	if !reflect.DeepEqual(schema["required"], []any{"config"}) {
		t.Fatalf("unexpected required inputs: %v", schema["required"])
	}
	descriptions := map[string]string{}
	var visit func(any)
	visit = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			if properties, ok := value["properties"].(map[string]any); ok {
				for name, property := range properties {
					if row, ok := property.(map[string]any); ok {
						descriptions[name], _ = row["description"].(string)
					}
				}
			}
			for _, child := range value {
				visit(child)
			}
		case []any:
			for _, child := range value {
				visit(child)
			}
		}
	}
	visit(schema)
	for _, name := range []string{"config", "required_inputs", "alerts", "threshold", "evaluation_seconds", "persistence_seconds", "incident_base_url", "entity_labels"} {
		if descriptions[name] == "" {
			t.Errorf("missing description for %s", name)
		}
	}
}

func TestArtifactSchemaNumericBounds(t *testing.T) {
	s := server.NewMCPServer("test", "1")
	RegisterArtifactTools(s, vmanomaly.NewClient("http://unused", "", nil))
	data, err := json.Marshal(s.GetTool("vmanomaly_generate_preset_artifacts").Tool)
	if err != nil {
		t.Fatal(err)
	}
	var tool struct {
		InputSchema struct {
			Properties map[string]struct {
				Properties map[string]map[string]any `json:"properties"`
				Required   []string                  `json:"required"`
			} `json:"properties"`
		} `json:"inputSchema"`
	}
	if err := json.Unmarshal(data, &tool); err != nil {
		t.Fatal(err)
	}
	alerts := tool.InputSchema.Properties["alerts"]
	if len(alerts.Required) != 0 {
		t.Fatalf("alert fields must remain optional: %v", alerts.Required)
	}
	for name, want := range map[string]map[string]any{
		"threshold":           {"type": "number", "exclusiveMinimum": float64(0)},
		"evaluation_seconds":  {"type": "integer", "minimum": float64(1), "maximum": float64(86400)},
		"persistence_seconds": {"type": "integer", "minimum": float64(0), "maximum": float64(604800)},
	} {
		t.Run(name, func(t *testing.T) {
			got := alerts.Properties[name]
			for _, key := range []string{"type", "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum"} {
				if !reflect.DeepEqual(got[key], want[key]) {
					t.Errorf("%s: got %v; want %v", key, got[key], want[key])
				}
			}
		})
	}
}
