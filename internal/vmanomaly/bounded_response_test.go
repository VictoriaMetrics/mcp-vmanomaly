package vmanomaly

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBoundedResponseIncludesErrorBodies(t *testing.T) {
	for _, status := range []int{200, 500} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status); w.Write([]byte("123456")) }))
		_, err := NewClient(s.URL, "", nil).doRequestLimited(context.Background(), http.MethodGet, "/", nil, 5)
		s.Close()
		if err == nil || err.Error() != "response exceeds byte budget" {
			t.Fatalf("unexpected result: %v", err)
		}
	}
}
