package vmanomaly

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoveryScopeAndWarnings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/prefix/api/v1/discovery" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("incorrect route/auth")
		}
		var req DiscoveryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if len(req.Match) != 2 || req.TenantID != "42:3" || req.Limit != 10 {
			t.Error("lost scope")
		}
		if _, err := w.Write([]byte(`{"completeness":"unknown","items":[],"warnings":["partial"]}`)); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	result, err := NewClient(server.URL+"/prefix", "fixture", nil).Discover(context.Background(), DiscoveryRequest{Operation: "metric_names", DatasourceURL: "https://vm.example", TenantID: "42:3", Match: []string{"a", "b"}, Start: 1, End: 100, Limit: 10})
	if err != nil || !json.Valid(result) {
		t.Fatalf("%s %v", result, err)
	}
	var got map[string]any
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatal(err)
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
