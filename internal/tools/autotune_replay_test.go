package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/vmanomaly"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestAutotuneReplayToolForwarding(t *testing.T) {
	replay := map[string]any{"fit_window": "3d", "fit_every": "1000w", "infer_every": "5m"}
	for _, withReplay := range []bool{false, true} {
		t.Run(map[bool]string{false: "omitted", true: "explicit"}[withReplay], func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				got, present := body["replay"]
				if present != withReplay || (withReplay && !reflect.DeepEqual(got, replay)) {
					t.Errorf("replay changed: %v", body)
				}
				if body["start"] != float64(100) || body["end"] != float64(200) || body["step"] != "1m" {
					t.Errorf("window/sampling changed: %v", body)
				}
				if _, err := w.Write([]byte(`{"task_id":"example","status":"running"}`)); err != nil {
					t.Error(err)
				}
			}))
			defer upstream.Close()
			s := server.NewMCPServer("test", "1")
			RegisterAnalysisTools(s, vmanomaly.NewClient(upstream.URL, "", nil))
			var req mcp.CallToolRequest
			args := map[string]any{"query": "up", "tuned_class_name": "mad_online", "start": 100, "end": 200, "step": "1m"}
			if withReplay {
				args["replay"] = replay
			}
			req.Params.Arguments = args
			result, err := s.GetTool("vmanomaly_create_autotune_task").Handler(context.Background(), req)
			if err != nil || result.IsError {
				t.Fatalf("%v %v", result, err)
			}
		})
	}
}

func TestAutotuneReplayBackendErrorIsNotRetried(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "invalid replay schedule", http.StatusUnprocessableEntity)
	}))
	defer upstream.Close()
	s := server.NewMCPServer("test", "1")
	RegisterAnalysisTools(s, vmanomaly.NewClient(upstream.URL, "", nil))
	var req mcp.CallToolRequest
	req.Params.Arguments = map[string]any{"query": "up", "tuned_class_name": "mad_online", "start": 100, "end": 200, "replay": map[string]any{"fit_window": "3d", "fit_every": "5m", "infer_every": "1h"}}
	result, err := s.GetTool("vmanomaly_create_autotune_task").Handler(context.Background(), req)
	if err != nil || !result.IsError || calls != 1 {
		t.Fatalf("unexpected fallback: calls=%d result=%v err=%v", calls, result, err)
	}
}

func TestAutotuneReplayValidationDoesNotSuggestBudgetRetry(t *testing.T) {
	for _, tc := range []struct {
		name     string
		start    any
		schedule map[string]any
		field    string
	}{
		{"missing start", nil, map[string]any{"fit_window": "1d", "fit_every": "1000w", "infer_every": "5m"}, "start"},
		{"missing fit window", 100, map[string]any{"fit_every": "1000w", "infer_every": "5m"}, "fit_window"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("invalid request reached backend") }))
			defer upstream.Close()
			s := server.NewMCPServer("test", "1")
			RegisterAnalysisTools(s, vmanomaly.NewClient(upstream.URL, "", nil))
			var req mcp.CallToolRequest
			args := map[string]any{"query": "up", "tuned_class_name": "mad_online", "end": 200, "replay": tc.schedule}
			if tc.start != nil {
				args["start"] = tc.start
			}
			req.Params.Arguments = args
			result, err := s.GetTool("vmanomaly_create_autotune_task").Handler(context.Background(), req)
			if err != nil || !result.IsError {
				t.Fatalf("expected tool error: %v %v", result, err)
			}
			text := result.Content[0].(mcp.TextContent).Text
			if !strings.Contains(text, tc.field) || strings.Contains(text, "budget") || strings.Contains(text, "sampled limit") {
				t.Fatalf("misleading validation guidance: %s", text)
			}
		})
	}
}
