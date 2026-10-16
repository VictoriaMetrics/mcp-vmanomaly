package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/vmanomaly"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestDiscoveryToolPreservesStructuredResultAndErrors(t *testing.T) {
	want := vmanomaly.DiscoveryRequest{Operation: "label_values", DatasourceURL: "https://vm.example", TenantID: "42:3", Match: []string{"up"}, Start: 1.5, End: 2.5, Label: "job", Limit: 5, TimeoutSeconds: 7, PassAuthHeaders: true}
	var status atomic.Int32
	status.Store(http.StatusOK)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got vmanomaly.DiscoveryRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("request mapping changed: %+v", got)
		}
		w.WriteHeader(int(status.Load()))
		if _, err := w.Write([]byte(`{"items":[],"completeness":"unknown","warnings":["limited"]}`)); err != nil {
			t.Error(err)
		}
	}))
	defer upstream.Close()
	s := server.NewMCPServer("test", "1")
	RegisterDiscoveryTools(s, vmanomaly.NewClient(upstream.URL, "", nil))
	tool := s.GetTool("vmanomaly_discover_metrics")
	if tool == nil || tool.Tool.Annotations.ReadOnlyHint == nil || !*tool.Tool.Annotations.ReadOnlyHint {
		t.Fatal("missing read-only tool")
	}
	var advertised struct {
		InputSchema struct {
			Required   []string `json:"required"`
			Properties map[string]struct {
				Description string   `json:"description"`
				Minimum     *float64 `json:"minimum"`
				Maximum     *float64 `json:"maximum"`
			} `json:"properties"`
		} `json:"inputSchema"`
	}
	metadata, err := json.Marshal(tool.Tool)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(metadata, &advertised); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"operation", "datasource_url", "match", "start", "end"} {
		if !slices.Contains(advertised.InputSchema.Required, field) {
			t.Errorf("missing required scope field: %s", field)
		}
	}
	for _, field := range []string{"operation", "datasource_url", "tenant_id", "match", "start", "end", "label", "limit", "timeout_seconds", "pass_auth_headers"} {
		if advertised.InputSchema.Properties[field].Description == "" {
			t.Errorf("missing description for %s", field)
		}
	}
	timeoutSchema := advertised.InputSchema.Properties["timeout_seconds"]
	if timeoutSchema.Minimum == nil || *timeoutSchema.Minimum != 0 || timeoutSchema.Maximum == nil || *timeoutSchema.Maximum != 30 {
		t.Fatalf("unexpected timeout schema bounds: %+v", timeoutSchema)
	}
	var request mcp.CallToolRequest
	request.Params.Arguments = map[string]any{"operation": want.Operation, "datasource_url": want.DatasourceURL, "tenant_id": want.TenantID, "match": want.Match, "start": want.Start, "end": want.End, "label": want.Label, "limit": want.Limit, "timeout_seconds": want.TimeoutSeconds, "pass_auth_headers": want.PassAuthHeaders}
	result, err := tool.Handler(context.Background(), request)
	if err != nil || result.IsError {
		t.Fatalf("%v %v", result, err)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil || string(encoded) != `{"items":[],"completeness":"unknown","warnings":["limited"]}` {
		t.Fatalf("structured response changed: %s %v", encoded, err)
	}
	status.Store(http.StatusNotFound)
	result, err = tool.Handler(context.Background(), request)
	if err != nil || !result.IsError {
		t.Fatal("unsupported endpoint appeared successful")
	}
}
