package vmanomaly

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestHeadersOverrideStaticHeaders(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "static-token", map[string]string{"X-Tenant": "static", "X-Static": "kept"})

	if _, err := client.GetHealth(context.Background()); err != nil {
		t.Fatalf("GetHealth failed: %v", err)
	}
	if got.Get("Authorization") != "Bearer static-token" || got.Get("X-Tenant") != "static" {
		t.Fatalf("static headers not sent without request headers: %v", got)
	}

	ctx := WithRequestHeaders(context.Background(), http.Header{
		"Authorization": {"Bearer user-token"},
		"x-tenant":      {"team-a"},
		"Content-Type":  {"text/plain"},
	})
	if _, err := client.GetHealth(ctx); err != nil {
		t.Fatalf("GetHealth failed: %v", err)
	}
	if got.Get("Authorization") != "Bearer user-token" {
		t.Errorf("Authorization = %q, want the forwarded token", got.Get("Authorization"))
	}
	if values := got.Values("X-Tenant"); len(values) != 1 || values[0] != "team-a" {
		t.Errorf("X-Tenant = %q, want only the forwarded team-a", values)
	}
	if got.Get("X-Static") != "kept" {
		t.Errorf("X-Static = %q, want kept", got.Get("X-Static"))
	}
	if got.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, forwarded headers must not override it", got.Get("Content-Type"))
	}
}

func TestWithRequestHeadersCopiesInput(t *testing.T) {
	headers := http.Header{"X-Tenant": {"team-a"}}
	ctx := WithRequestHeaders(context.Background(), headers)
	headers.Set("X-Tenant", "mutated")
	if got := requestHeadersFromContext(ctx).Get("X-Tenant"); got != "team-a" {
		t.Fatalf("stored header = %q, want team-a", got)
	}
	if WithRequestHeaders(ctx, nil) != ctx {
		t.Fatal("empty headers must return the context unchanged")
	}
}
