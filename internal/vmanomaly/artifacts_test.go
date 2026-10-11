package vmanomaly

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestArtifactRequestPreservesMappingsAndZeroPersistence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/preset/artifacts" || r.Method != "POST" {
			t.Error("incorrect route")
		}
		var input map[string]any
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		if input["alerts"].(map[string]any)["persistence_seconds"] != float64(0) {
			t.Error("lost explicit zero")
		}
		if len(input["config"].(map[string]any)["models"].(map[string]any)) != 2 {
			t.Error("lost mappings")
		}
		w.Write([]byte(`{"schema_version":1,"validation":"schema-valid","warnings":["not live-tested"]}`))
	}))
	defer server.Close()
	zero := 0
	result, err := NewClient(server.URL, "", nil).GenerateArtifacts(context.Background(), ArtifactRequest{
		Config: map[string]any{"models": map[string]any{"a": map[string]any{"queries": []string{"cpu"}}, "b": map[string]any{"queries": []string{"mem"}}}},
		Alerts: &ArtifactAlertPolicy{PersistenceSeconds: &zero},
	})
	if err != nil || !json.Valid(result) {
		t.Fatalf("%s %v", result, err)
	}
}

func TestArtifactErrorsAreNotSuccessfulEmptyBundles(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	client := NewClient(server.URL, "", nil)
	if _, err := client.GenerateArtifacts(context.Background(), ArtifactRequest{}); err == nil {
		t.Fatal("missing config accepted")
	}
	if _, err := client.GenerateArtifacts(context.Background(), ArtifactRequest{Config: map[string]any{}}); err == nil {
		t.Fatal("404 accepted")
	}
}
