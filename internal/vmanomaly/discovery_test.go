package vmanomaly

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestDiscoveryScopeAndWarnings(t *testing.T) {
	want := DiscoveryRequest{Operation: "metric_names", DatasourceURL: "https://vm.example", TenantID: "42:3", Match: []string{"a", "b"}, Start: 1, End: 100, Limit: 10}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/prefix/api/v1/discovery" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("incorrect route/auth")
		}
		var req DiscoveryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(req, want) {
			t.Error("lost scope")
		}
		if _, err := w.Write([]byte(`{"completeness":"unknown","items":[],"warnings":["partial"]}`)); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	result, err := NewClient(server.URL+"/prefix", "fixture", nil).Discover(context.Background(), want)
	if err != nil || !json.Valid(result) {
		t.Fatalf("%s %v", result, err)
	}
	var got map[string]any
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got["warnings"], []any{"partial"}) {
		t.Fatalf("lost warnings: %v", got["warnings"])
	}
	if got["completeness"] != "unknown" {
		t.Fatal("lost completeness")
	}
}

func TestDiscoveryUnsupportedAndCanceled(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	client := NewClient(server.URL, "", nil)
	req := DiscoveryRequest{Operation: "series", Match: []string{"a"}, Start: 1, End: 2}
	if _, err := client.Discover(context.Background(), req); err == nil {
		t.Fatal("404 was accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Discover(ctx, req); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestDiscoveryRequiresLabelBeforeDispatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid label request reached upstream")
	}))
	defer server.Close()
	for _, label := range []string{"", " ", "\t\n"} {
		request := DiscoveryRequest{Operation: "label_values", DatasourceURL: "https://vm.example", Match: []string{"up"}, Start: 1, End: 2, Label: label}
		if _, err := NewClient(server.URL, "", nil).Discover(context.Background(), request); err == nil {
			t.Errorf("blank label accepted: %q", label)
		}
	}
}

func TestDiscoveryValidationErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid request reached upstream")
	}))
	defer server.Close()
	client := NewClient(server.URL, "", nil)
	tests := []struct {
		name    string
		change  func(*DiscoveryRequest)
		message string
	}{
		{"operation", func(r *DiscoveryRequest) { r.Operation = "bad" }, "operation must be"},
		{"missing selectors", func(r *DiscoveryRequest) { r.Match = nil }, "match must contain at least one selector"},
		{"too many selectors", func(r *DiscoveryRequest) { r.Match = make([]string, maxDiscoverySelectors+1) }, "match contains 11 selectors; maximum is 10"},
		{"unordered window", func(r *DiscoveryRequest) { r.End = r.Start }, "end must be greater than start"},
		{"long window", func(r *DiscoveryRequest) { r.End = r.Start + maxDiscoveryWindowSeconds + 1 }, "discovery window exceeds"},
		{"negative limit", func(r *DiscoveryRequest) { r.Limit = -1 }, "got -1"},
		{"large limit", func(r *DiscoveryRequest) { r.Limit = maxDiscoveryResults + 1 }, "got 1001"},
		{"blank selector", func(r *DiscoveryRequest) { r.Match = []string{"up", " "} }, "match[1] must not be blank"},
		{"long selector", func(r *DiscoveryRequest) { r.Match = []string{strings.Repeat("a", maxDiscoverySelectorBytes+1)} }, "match[0] exceeds 4096 bytes (4097 bytes)"},
		{"JSON encoding", func(r *DiscoveryRequest) { r.Start = math.NaN() }, "cannot encode discovery request as JSON"},
		{"JSON budget", func(r *DiscoveryRequest) { r.TenantID = strings.Repeat("a", maxDiscoveryRequestBytes) }, "discovery request exceeds 65536 bytes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := DiscoveryRequest{Operation: "series", Match: []string{"up"}, Start: 1, End: 2}
			tt.change(&request)
			_, err := client.Discover(context.Background(), request)
			if err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("want %q; got %v", tt.message, err)
			}
			if tt.name == "JSON encoding" {
				var original *json.UnsupportedValueError
				if !errors.As(err, &original) {
					t.Fatalf("lost marshal error: %v", err)
				}
			}
			if tt.name == "JSON budget" {
				body, marshalErr := json.Marshal(request)
				if marshalErr != nil {
					t.Fatal(marshalErr)
				}
				if !strings.Contains(err.Error(), fmt.Sprintf("(%d bytes)", len(body))) {
					t.Fatalf("missing actual size: %v", err)
				}
			}
		})
	}
}

func TestDiscoveryScopeBoundaries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(`{"items":[]}`)); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	request := DiscoveryRequest{Operation: "series", Start: 0, End: maxDiscoveryWindowSeconds, Limit: maxDiscoveryResults}
	for range maxDiscoverySelectors {
		request.Match = append(request.Match, strings.Repeat("a", maxDiscoverySelectorBytes))
	}
	for _, limit := range []int{0, 1, maxDiscoveryResults} {
		request.Limit = limit
		if _, err := NewClient(server.URL, "", nil).Discover(context.Background(), request); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDiscoveryTimeoutBounds(t *testing.T) {
	for _, timeout := range []float64{-1, 31, math.NaN(), math.Inf(1), math.Inf(-1)} {
		t.Run(fmt.Sprint(timeout), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("invalid timeout reached upstream")
			}))
			defer server.Close()
			request := DiscoveryRequest{Operation: "series", Match: []string{"up"}, Start: 1, End: 2, TimeoutSeconds: timeout}
			if _, err := NewClient(server.URL, "", nil).Discover(context.Background(), request); err == nil || !strings.Contains(err.Error(), "timeout_seconds") {
				t.Fatalf("expected timeout error; got %v", err)
			}
		})
	}
	for _, timeout := range []float64{0, 0.1, maxDiscoveryTimeoutSeconds} {
		t.Run(fmt.Sprint(timeout), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				value, present := body["timeout_seconds"]
				if timeout == 0 && present {
					t.Error("default timeout must be omitted")
				}
				if timeout != 0 && value != timeout {
					t.Errorf("timeout changed: %v", value)
				}
				if _, err := w.Write([]byte(`{"items":[]}`)); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			request := DiscoveryRequest{Operation: "series", Match: []string{"up"}, Start: 1, End: 2, TimeoutSeconds: timeout}
			if _, err := NewClient(server.URL, "", nil).Discover(context.Background(), request); err != nil {
				t.Fatal(err)
			}
		})
	}
}
