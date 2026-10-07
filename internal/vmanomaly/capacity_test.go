package vmanomaly

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCapacityParallelCallsRespectServerAdmission(t *testing.T) {
	var mu sync.Mutex
	var starts []time.Time
	var active atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if active.Add(1) != 1 {
			t.Error("overlapping capacity requests")
		}
		defer active.Add(-1)
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	client := NewClient(server.URL, "", nil)
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			if _, err := client.Capacity(context.Background(), "throughput", map[string]any{}); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if len(starts) != 3 {
		t.Fatalf("expected three calls, got %d", len(starts))
	}
	for i := 1; i < len(starts); i++ {
		if starts[i].Sub(starts[i-1]) < 100*time.Millisecond {
			t.Error("calls violate server minimum interval")
		}
	}
}

func TestCapacityValidationDetailsAndRecovery(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"detail":"required model parameters missing: period"}`))
			return
		}
		_, _ = w.Write([]byte(`{"estimated_max_models":100}`))
	}))
	defer server.Close()
	client := NewClient(server.URL, "", nil)
	_, err := client.Capacity(context.Background(), "throughput", map[string]any{})
	want := `API error (status 422): {"detail":"required model parameters missing: period"}`
	if err == nil || err.Error() != want {
		t.Fatalf("expected compact validation detail, got %v", err)
	}
	if _, err := client.Capacity(context.Background(), "throughput", map[string]any{}); err != nil {
		t.Fatalf("follow-up after validation failure: %v", err)
	}
}

func TestCapacityAdmissionCancellationAndBounds(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	client := NewClientWithTimeout(server.URL, "", nil, 5*time.Second)
	// Hold the active slot to exercise queue timeout without depending on HTTP timing.
	release, err := client.admitCapacity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	_, err = client.Capacity(ctx, "throughput", map[string]any{})
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected admission deadline, got %v", err)
	}
	if len(client.capacityQueue) != 1 {
		t.Fatal("timed-out waiter leaked queue occupancy")
	}
	// Discovery is available while projections are occupied.
	if _, err = client.Capacity(context.Background(), "profiles", nil); err != nil {
		t.Fatal(err)
	}
	for len(client.capacityQueue) < cap(client.capacityQueue) {
		client.capacityQueue <- struct{}{}
	}
	if _, err = client.Capacity(context.Background(), "estimate", map[string]any{}); err == nil {
		t.Fatal("unbounded queue")
	}
	for len(client.capacityQueue) > 1 {
		<-client.capacityQueue
	}
	release()
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	if _, err = client.Capacity(ctx, "throughput", map[string]any{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if len(client.capacityQueue) != 0 || len(client.capacitySlot) != 0 {
		t.Fatal("canceled waiter leaked admission")
	}
	if calls.Load() != 1 {
		t.Fatalf("waiting/canceled requests reached HTTP: %d", calls.Load())
	}
	// The pacing pause also consumes the same request deadline.
	client.capacityNext = time.Now().Add(time.Hour)
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Millisecond)
	_, err = client.Capacity(ctx, "throughput", map[string]any{})
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected pacing deadline, got %v", err)
	}
	if len(client.capacityQueue) != 0 || len(client.capacitySlot) != 0 {
		t.Fatal("pacing timeout leaked admission")
	}
	// After the pacing window, a later request can use the released slot.
	client.capacityNext = time.Time{}
	if _, err = client.Capacity(context.Background(), "throughput", map[string]any{}); err != nil {
		t.Fatal(err)
	}
}

// Deliberately private: public errors must not echo arbitrary marshaler failures.
type failingCapacityValue struct{}

func (failingCapacityValue) MarshalJSON() ([]byte, error) {
	return nil, errors.New("private fixture detail")
}

func TestCapacityRejectsInvalidPayloadBeforeAdmission(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	client := NewClient(server.URL, "", nil)
	for _, operation := range []string{"estimate", "throughput"} {
		for _, tc := range []struct {
			name    string
			payload map[string]any
			want    string
		}{
			{"nil", nil, "capacity request must be a JSON object"},
			{"unsupported", map[string]any{"nested": map[string]any{"value": make(chan int)}}, "capacity request contains values that cannot be encoded as JSON"},
			{"custom error", map[string]any{"value": failingCapacityValue{}}, "capacity request contains values that cannot be encoded as JSON"},
			{"oversized", map[string]any{"data": strings.Repeat("x", 65536)}, "capacity request exceeds 64 KiB (65547 bytes)"},
		} {
			t.Run(operation+"/"+tc.name, func(t *testing.T) {
				_, err := client.Capacity(context.Background(), operation, tc.payload)
				if err == nil || err.Error() != tc.want {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(client.capacityQueue) != 0 || len(client.capacitySlot) != 0 {
					t.Fatal("rejected payload acquired admission")
				}
			})
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid requests reached HTTP: %d", calls.Load())
	}
	if _, err := client.Capacity(context.Background(), "estimate", map[string]any{}); err != nil {
		t.Fatal(err)
	}
}

func TestCapacityClientTimeoutIncludesAdmission(t *testing.T) {
	client := NewClientWithTimeout("http://invalid", "", nil, 30*time.Millisecond)
	release, err := client.admitCapacity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err = client.Capacity(context.Background(), "throughput", map[string]any{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("configured timeout must include admission: %v", err)
	}
	if len(client.capacityQueue) != 1 {
		t.Fatal("timed-out waiter leaked admission")
	}
}
