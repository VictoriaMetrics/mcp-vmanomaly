package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/vmanomaly"
	"github.com/mark3labs/mcp-go/mcp"
)

func capacityCall(arguments map[string]any) mcp.CallToolRequest {
	var request mcp.CallToolRequest
	request.Params.Arguments = arguments
	return request
}

func TestCapacityForwardAndReverse(t *testing.T) {
	for _, operation := range []string{"estimate", "throughput"} {
		t.Run(operation, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/api/v1/deployment-sizing/"+operation || r.Header.Get("Authorization") != "Bearer test-token" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				if body["vmanomaly_version"] != "v1.31.0" {
					t.Errorf("options lost: %v", body)
				}
				if _, leaked := body["options"]; leaked {
					t.Error("MCP wrapper leaked to API")
				}
				if operation == "throughput" {
					if body["cpus"] != float64(2) || body["ram_limit_bytes"] != float64(2147483648) {
						t.Errorf("wrong sizing: %v", body)
					}
				} else {
					workload := body["workloads"].([]any)[0].(map[string]any)
					if workload["entity_count"] != float64(1000) || workload["infer_points_per_cycle"] != float64(2) {
						t.Errorf("wrong workload: %v", workload)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"experimental":true,"warnings":["extrapolated"],"resolved_request":{},"limiting_constraint":"ram"}`))
			}))
			defer server.Close()
			client := vmanomaly.NewClient(server.URL, "test-token", nil)
			args := map[string]any{"options": map[string]any{"vmanomaly_version": "v1.31.0"}}
			handler := handleCapacityEstimate(client)
			if operation == "estimate" {
				args["workloads"] = []any{map[string]any{"model_class": "mad_online", "entity_count": 1000, "infer_every_seconds": 300, "options": map[string]any{"infer_points_per_cycle": 2}}}
			} else {
				handler = handleCapacityThroughput(client)
				args["model_class"] = "mad_online"
				args["cpus"] = 2
				args["infer_every_seconds"] = 300
				args["ram_limit_bytes"] = 2147483648
			}
			result, err := handler(context.Background(), capacityCall(args))
			if err != nil || result.IsError {
				t.Fatalf("unexpected error: %v %v", result, err)
			}
			text := result.Content[0].(mcp.TextContent).Text
			if !strings.Contains(text, "Experimental estimate") || !strings.Contains(text, "limiting_constraint") {
				t.Fatal("response details lost")
			}
		})
	}
}

func TestCapacityRejectsAmbiguousOverrides(t *testing.T) {
	client := vmanomaly.NewClient("http://invalid", "", nil)
	result, err := handleCapacityThroughput(client)(context.Background(), capacityCall(map[string]any{
		"model_class": "mad_online", "cpus": 2, "infer_every_seconds": 300, "options": map[string]any{"cpus": 8},
	}))
	if err != nil || !result.IsError || !strings.Contains(result.Content[0].(mcp.TextContent).Text, "override") {
		t.Fatalf("expected local rejection: %v %v", result, err)
	}
}

func TestPeerCapacityNoticeIsConcise(t *testing.T) {
	for _, shape := range []string{`{"topology":"wide"}`, `{"workloads":[{"topology":"wide"}]}`} {
		data := []byte(`{"resolved_request":` + shape + `,"warnings":["Workload: experimental peer estimate for fixed equal-size pools. Worker speedup uses the shared runtime fallback.","Workload: larger peer pools use an unvalidated n log(n) CPU approximation.","Datasource download time is not measured."]}`)
		var result map[string]any
		if err := json.Unmarshal(capacityDisplay(data), &result); err != nil {
			t.Fatal(err)
		}
		if result["estimate_notice"] != "Rough peer-group estimate; allow extra headroom and validate with your workload." {
			t.Fatalf("missing peer notice: %v", result)
		}
		warnings := fmt.Sprint(result["warnings"])
		if strings.Contains(warnings, "n log(n)") || strings.Contains(warnings, "Worker speedup") || !strings.Contains(warnings, "fixed equal-size pools") || !strings.Contains(warnings, "Datasource download") {
			t.Fatalf("wrong warning filtering: %s", warnings)
		}
	}
}

func TestPeerCapacityPreservesPoolShapeAndLimitations(t *testing.T) {
	for _, operation := range []string{"estimate", "throughput"} {
		t.Run(operation, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				workload := body
				if operation == "estimate" {
					workload = body["workloads"].([]any)[0].(map[string]any)
					if workload["entity_count"] != float64(100) {
						t.Errorf("pool count changed: %v", workload)
					}
				}
				if workload["model_class"] != "peer_outlier" || workload["topology"] != "wide" || workload["channels_per_entity"] != float64(100) {
					t.Errorf("peer shape lost: %v", workload)
				}
				_, _ = w.Write([]byte(`{"experimental":true,"input_series":10000,"warnings":["Experimental peer estimate: fixed equal-size pools, no membership churn; shared fallback worker scaling."]}`))
			}))
			defer server.Close()
			client := vmanomaly.NewClient(server.URL, "", nil)
			options := map[string]any{"topology": "wide", "channels_per_entity": 100}
			args := map[string]any{"model_class": "peer_outlier", "cpus": 2, "infer_every_seconds": 300, "options": options}
			handler := handleCapacityThroughput(client)
			if operation == "estimate" {
				handler = handleCapacityEstimate(client)
				args = map[string]any{"workloads": []any{map[string]any{"model_class": "peer_outlier", "entity_count": 100, "infer_every_seconds": 300, "options": options}}}
			}
			result, err := handler(context.Background(), capacityCall(args))
			if err != nil || result.IsError {
				t.Fatalf("unexpected result: %v %v", result, err)
			}
			text := result.Content[0].(mcp.TextContent).Text
			if !strings.Contains(text, "fixed equal-size pools") || !strings.Contains(text, "shared fallback worker scaling") || !strings.Contains(text, `"input_series":10000`) {
				t.Fatalf("peer limitations or counts lost: %s", text)
			}
		})
	}
}

func TestCapacityServerErrorsAreNotRetried(t *testing.T) {
	for _, status := range []int{404, 422, 429} {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"detail":"test rejection"}`))
		}))
		result, err := handleCapacityThroughput(vmanomaly.NewClient(server.URL, "", nil))(context.Background(), capacityCall(map[string]any{"model_class": "mad_online", "cpus": 2, "infer_every_seconds": 300}))
		server.Close()
		if err != nil || !result.IsError || calls != 1 {
			t.Fatalf("status %d: %v %v calls=%d", status, result, err, calls)
		}
	}
}

func TestCapacityProfilesAndBounds(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/api/v1/deployment-sizing/profiles" {
			t.Error("wrong catalog route")
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	client := vmanomaly.NewClient(server.URL, "", nil)
	if _, err := client.Capacity(context.Background(), "profiles", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Capacity(context.Background(), "../other", nil); err == nil {
		t.Fatal("accepted arbitrary path")
	}
	if _, err := client.Capacity(context.Background(), "estimate", map[string]any{"oversized": strings.Repeat("x", 65536)}); err == nil {
		t.Fatal("accepted oversized body")
	}
	if calls != 1 {
		t.Fatalf("unexpected backend calls: %d", calls)
	}
}

func TestThroughputDefaultsToNewSamplesAndPreservesExplicitBatch(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				expected := float64(5)
				if explicit {
					expected = 2
				}
				if body["infer_points_per_cycle"] != expected {
					t.Errorf("unexpected scored points: %v", body["infer_points_per_cycle"])
				}
				_, _ = w.Write([]byte(`{"experimental":true}`))
			}))
			defer server.Close()
			options := map[string]any{"history": map[string]any{"step_seconds": 60, "window_seconds": 1209600, "fit_every_seconds": 86400}}
			if explicit {
				options["infer_points_per_cycle"] = 2
			}
			result, err := handleCapacityThroughput(vmanomaly.NewClient(server.URL, "", nil))(context.Background(), capacityCall(map[string]any{
				"model_class": "mad_online", "cpus": 2, "infer_every_seconds": 300, "options": options,
			}))
			if err != nil || result.IsError {
				t.Fatalf("failed: %v %v", result, err)
			}
			expectedBlocks := 2
			if explicit {
				expectedBlocks = 1
			}
			if len(result.Content) != expectedBlocks {
				t.Fatal("default assumption must be disclosed only when applied")
			}
		})
	}
}

func TestCapacityDisplayKeepsOperationalValuesAndHidesCalibration(t *testing.T) {
	raw := []byte(`{"experimental":true,"calibration":{"model_extrapolation_factor":2500},"calibration_profile":"private-sample","resolved_request":{"model_params":{"calibration":"preserve user input"}},"recommended":{"planned_ram_bytes_per_node":9007199254740993,"amdahl_efficiency":0.4,"fit":{"planned_seconds":70},"online_fit":{"soft_budget_exceeded":true}},"candidates":[{"amdahl_efficiency":0.4}],"workloads":[{"entity_count":20000,"is_online":true,"fit_every_seconds":null,"calibration":{"probe_models":8},"fit_amdahl_serial_fraction":0.1}],"warnings":["Models extrapolate 2500x","Fit exceeds the cadence","Target version has no exact calibration"],"assumptions":["Amdahl uses sampled CPU costs","Datasource execution/download is excluded"]}`)
	output := string(capacityDisplay(raw))
	for _, omitted := range []string{"2500", "probe_models", "private-sample", "amdahl_efficiency", "fit_amdahl", "sampled CPU"} {
		if strings.Contains(output, omitted) {
			t.Errorf("internal detail leaked: %s", omitted)
		}
	}
	for _, retained := range []string{"9007199254740993", "20000", "planned_seconds", "soft_budget_exceeded", "preserve user input", "Fit exceeds", "Target version", "Datasource", "Experimental estimate"} {
		if !strings.Contains(output, retained) {
			t.Errorf("operational data lost: %s", retained)
		}
	}
	if string(capacityDisplay([]byte("unexpected non-JSON"))) != "unexpected non-JSON" {
		t.Fatal("unexpected response changed")
	}
}

func TestForwardResolvesCycleAndPreservesQueryPlacementOnFailure(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			calls := 0
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				row := body["workloads"].([]any)[0].(map[string]any)
				points, deadline := float64(4), float64(120)
				if explicit {
					points, deadline = 24, 90
				}
				if row["infer_points_per_cycle"] != points || row["query_count"] != float64(4) || row["entity_count"] != float64(1000000) {
					t.Errorf("workload changed: %v", row)
				}
				policy := body["policy"].(map[string]any)
				if policy["infer_budget_seconds"] != deadline || len(policy["cpu_candidates"].([]any)) != 3 {
					t.Errorf("wrong search: %v", policy)
				}
				deployment := body["deployment"].(map[string]any)
				if deployment["members_count"] != float64(4) || deployment["split_by"] != "queries" {
					t.Errorf("placement lost: %v", deployment)
				}
				if _, exists := row["fit"].(map[string]any)["fit_every_seconds"]; exists {
					t.Error("invented refit")
				}
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(`{"detail":{"code":"deployment_infeasible","checked_candidates":[{"violations":[{"constraint":"inference_deadline","actual":162,"limit":120,"unit":"seconds"}]}]}}`))
			}))
			defer backend.Close()
			options := map[string]any{"query_count": 4, "fit": map[string]any{"window_seconds": 604800, "step_seconds": 30}}
			policy := map[string]any{"cpu_candidates": []int{16, 32, 64}, "max_cpus_per_node": 64}
			if explicit {
				options["infer_points_per_cycle"] = 24
				policy["infer_budget_seconds"] = 90
			}
			result, err := handleCapacityEstimate(vmanomaly.NewClient(backend.URL, "", nil))(context.Background(), capacityCall(map[string]any{
				"workloads": []any{map[string]any{"model_class": "mad", "entity_count": 1000000, "infer_every_seconds": 120, "options": options}},
				"options":   map[string]any{"policy": policy, "deployment": map[string]any{"members_count": 4, "split_by": "queries"}},
			}))
			if err != nil || !result.IsError || calls != 1 {
				t.Fatalf("failure must not trigger replanning: %v %v %d", result, err, calls)
			}
			message := result.Content[0].(mcp.TextContent).Text
			if !strings.Contains(message, "inference_deadline") || !strings.Contains(message, "162") {
				t.Fatal("binding constraint lost")
			}
			blocks := 2
			if explicit {
				blocks = 1
			}
			if len(result.Content) != blocks {
				t.Fatal("derived assumptions must accompany failure too")
			}
		})
	}
}

func TestForwardMixedCadencesAndExplicitPolicyDefaults(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["policy"].(map[string]any)["infer_budget_seconds"] != float64(61) {
			t.Error("deadline is not shortest cadence")
		}
		rows := body["workloads"].([]any)
		if rows[1].(map[string]any)["infer_points_per_cycle"] != float64(3) {
			t.Error("point count must round upward")
		}
		_, _ = w.Write([]byte(`{"experimental":true,"placement":"hypothetical_balanced_partition"}`))
	}))
	defer backend.Close()
	args := map[string]any{"workloads": []any{
		map[string]any{"model_class": "mad", "entity_count": 10, "infer_every_seconds": 120, "options": map[string]any{"alias": "one"}},
		map[string]any{"model_class": "mad", "entity_count": 10, "infer_every_seconds": 61, "options": map[string]any{"alias": "two", "fit": map[string]any{"window_seconds": 600, "step_seconds": 30}}},
	}}
	result, err := handleCapacityEstimate(vmanomaly.NewClient(backend.URL, "", nil))(context.Background(), capacityCall(args))
	if err != nil || result.IsError {
		t.Fatalf("unexpected error %v %v", result, err)
	}
	args["options"] = map[string]any{"policy": nil}
	result, err = handleCapacityEstimate(vmanomaly.NewClient("http://invalid", "", nil))(context.Background(), capacityCall(args))
	if err != nil || !result.IsError {
		t.Fatal("invalid explicit policy must not be silently replaced")
	}
}

func TestThroughputOptionalRAMContract(t *testing.T) {
	for _, supplied := range []bool{false, true} {
		t.Run(fmt.Sprint(supplied), func(t *testing.T) {
			calls := 0
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				value, exists := body["ram_limit_bytes"]
				if exists != supplied || (supplied && value != float64(2147483648)) {
					t.Errorf("unexpected RAM field: %v, present=%v", value, exists)
				}
				if body["topology"] != "multivariate" || body["channels_per_entity"] != float64(3) {
					t.Errorf("group shape lost: %v", body)
				}
				_, _ = w.Write([]byte(`{"estimated_max_models":10,"input_series":30}`))
			}))
			defer backend.Close()
			args := map[string]any{"model_class": "temporal_envelope_multivariate", "cpus": 2, "infer_every_seconds": 120, "options": map[string]any{"topology": "multivariate", "channels_per_entity": 3}}
			if supplied {
				args["ram_limit_bytes"] = int64(2147483648)
			}
			handler := handleCapacityThroughput(vmanomaly.NewClient(backend.URL, "", nil))
			result, err := handler(context.Background(), capacityCall(args))
			if err != nil || result.IsError {
				t.Fatalf("unexpected result: %v %v", result, err)
			}
			var output map[string]any
			if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &output); err != nil {
				t.Fatal(err)
			}
			if output["estimated_max_models"] != float64(10) || output["input_series"] != float64(30) {
				t.Fatal("model and input-series counts were changed")
			}
			args["options"].(map[string]any)["ram_limit_bytes"] = 1
			result, err = handler(context.Background(), capacityCall(args))
			if err != nil || !result.IsError || calls != 1 {
				t.Fatal("RAM override must be rejected even when named RAM is absent")
			}
		})
	}
}

func TestCapacityProfilesObjectRemainsUnchanged(t *testing.T) {
	const raw = `{"profiles":[{"id":"example","model_class":"mad_online"}],"calibration":{"version":"1.31.0"},"calibration_profile":"catalog","warnings":["catalog note"]}`
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/deployment-sizing/profiles" {
			t.Error("unexpected catalog request")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) != 0 {
			t.Error("profile discovery must have no body")
		}
		_, _ = w.Write([]byte(raw))
	}))
	defer backend.Close()
	result, err := capacityResult(context.Background(), vmanomaly.NewClient(backend.URL, "", nil), "profiles", nil)
	if err != nil || result.IsError || result.Content[0].(mcp.TextContent).Text != raw {
		t.Fatalf("catalog modified: %v %v", result, err)
	}
}

func TestThroughputFailureRetainsDerivedAssumption(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			calls := 0
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				want := float64(4)
				if explicit {
					want = 2
				}
				if body["infer_points_per_cycle"] != want {
					t.Error("wrong point count")
				}
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(`{"detail":"test rejection"}`))
			}))
			defer backend.Close()
			options := map[string]any{"history": map[string]any{"window_seconds": 604800, "step_seconds": 30}}
			if explicit {
				options["infer_points_per_cycle"] = 2
			}
			result, err := handleCapacityThroughput(vmanomaly.NewClient(backend.URL, "", nil))(context.Background(), capacityCall(map[string]any{"model_class": "mad_online", "cpus": 2, "infer_every_seconds": 120, "options": options}))
			if err != nil || !result.IsError || calls != 1 {
				t.Fatalf("unexpected rejection: %v %v calls=%d", result, err, calls)
			}
			wantBlocks := 2
			if explicit {
				wantBlocks = 1
			}
			if len(result.Content) != wantBlocks {
				t.Fatal("derived assumptions must accompany failures")
			}
			if !explicit && !strings.Contains(result.Content[1].(mcp.TextContent).Text, "infer_points_per_cycle=4") {
				t.Fatal("resolved assumption missing")
			}
		})
	}
}
