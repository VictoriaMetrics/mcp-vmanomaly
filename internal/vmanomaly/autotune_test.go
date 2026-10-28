package vmanomaly

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInvalidAutotuneReplayNeverReachesBackend(t *testing.T) {
	number := func(v float64) *float64 { return &v }
	for _, tc := range []struct {
		name       string
		start, end *float64
		replay     AutotuneReplayConfig
	}{
		{"missing start", nil, number(2), AutotuneReplayConfig{"1d", "1000w", "5m"}},
		{"missing end", number(1), nil, AutotuneReplayConfig{"1d", "1000w", "5m"}},
		{"equal bounds", number(1), number(1), AutotuneReplayConfig{"1d", "1000w", "5m"}},
		{"reversed bounds", number(2), number(1), AutotuneReplayConfig{"1d", "1000w", "5m"}},
		{"nan", number(math.NaN()), number(2), AutotuneReplayConfig{"1d", "1000w", "5m"}},
		{"infinite", number(1), number(math.Inf(1)), AutotuneReplayConfig{"1d", "1000w", "5m"}},
		{"negative infinity", number(math.Inf(-1)), number(2), AutotuneReplayConfig{"1d", "1000w", "5m"}},
		{"blank fit window", number(1), number(2), AutotuneReplayConfig{" ", "1000w", "5m"}},
		{"blank infer interval", number(1), number(2), AutotuneReplayConfig{"1d", "1000w", "\t"}},
		{"empty schedule", number(1), number(2), AutotuneReplayConfig{}},
		{"blank duration", number(1), number(2), AutotuneReplayConfig{"1d", " ", "5m"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("invalid replay reached backend") }))
			defer upstream.Close()
			_, err := NewClient(upstream.URL, "", nil).CreateAutotuneTask(context.Background(), &AutotuneTaskRequest{Start: tc.start, End: tc.end, Replay: &tc.replay})
			if !errors.Is(err, ErrInvalidAutotuneReplay) {
				t.Fatalf("expected replay validation error, got %v", err)
			}
		})
	}
}

func TestNilAutotuneRequestPreservesBackendHandling(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "request required", http.StatusUnprocessableEntity)
	}))
	defer upstream.Close()
	_, err := NewClient(upstream.URL, "", nil).CreateAutotuneTask(context.Background(), nil)
	if err == nil || errors.Is(err, ErrInvalidAutotuneReplay) || calls != 1 {
		t.Fatalf("nil request did not preserve backend handling: calls=%d err=%v", calls, err)
	}
}
