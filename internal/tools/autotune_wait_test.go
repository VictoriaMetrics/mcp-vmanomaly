package tools

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/vmanomaly"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestAutotuneWaitCompletionAndImmediateStatus(t *testing.T) {
	for _, wait := range []time.Duration{0, time.Second} {
		calls := 0
		result, err := waitAutotuneTask(context.Background(), func(context.Context) (*vmanomaly.AutotuneTaskStatus, error) {
			calls++
			status := "running"
			if calls == 2 {
				status = "done"
			}
			return &vmanomaly.AutotuneTaskStatus{Status: status}, nil
		}, wait, time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		if wait == 0 && (calls != 1 || result.Status != "running") {
			t.Fatal("immediate mode polled")
		}
		if wait != 0 && (calls != 2 || result.Status != "done") {
			t.Fatal("completion not returned")
		}
	}
}

func TestAutotuneWaitTimeoutAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	get := func(context.Context) (*vmanomaly.AutotuneTaskStatus, error) {
		return &vmanomaly.AutotuneTaskStatus{Status: "running"}, nil
	}
	result, err := waitAutotuneTask(ctx, get, time.Millisecond, time.Hour)
	if err != nil || result.Status != "running" {
		t.Fatal("deadline must return last status", err)
	}
	cancel()
	_, err = waitAutotuneTask(ctx, get, time.Second, time.Hour)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("parent cancellation lost", err)
	}
}

func TestAutotuneWaitStopsOnTerminalFailure(t *testing.T) {
	result, err := waitAutotuneTask(context.Background(), func(context.Context) (*vmanomaly.AutotuneTaskStatus, error) {
		return &vmanomaly.AutotuneTaskStatus{Status: "error"}, nil
	}, time.Second, time.Hour)
	if err != nil || result.Status != "error" {
		t.Fatal("terminal failure not returned", err)
	}
}

func TestAutotuneWaitRejectsInvalidLimitsBeforeCallingBackend(t *testing.T) {
	for _, seconds := range []int{-1, 11} {
		result, err := handleGetAutotuneTask(nil)(context.Background(), mcp.CallToolRequest{}, AutotuneTaskStatusArgs{TaskID: "task", WaitSeconds: seconds})
		if err != nil || !result.IsError {
			t.Fatal("invalid wait was not rejected", err)
		}
	}
}

func TestAutotuneWaitPropagatesBackendFailure(t *testing.T) {
	expected := errors.New("backend failure")
	_, err := waitAutotuneTask(context.Background(), func(context.Context) (*vmanomaly.AutotuneTaskStatus, error) {
		return nil, expected
	}, time.Second, time.Hour)
	if !errors.Is(err, expected) {
		t.Fatal("backend failure lost", err)
	}
}

func TestAutotuneWaitErrorAtDeadline(t *testing.T) {
	for _, deadlineError := range []bool{false, true} {
		backendError := errors.New("backend failed at deadline")
		calls := 0
		result, err := waitAutotuneTask(context.Background(), func(ctx context.Context) (*vmanomaly.AutotuneTaskStatus, error) {
			calls++
			if calls == 1 {
				return &vmanomaly.AutotuneTaskStatus{Status: "running"}, nil
			}
			<-ctx.Done()
			if deadlineError {
				return nil, ctx.Err()
			}
			return nil, backendError
		}, 100*time.Millisecond, 0)
		if calls != 2 {
			t.Fatalf("expected in-flight request at deadline, got %d calls", calls)
		}
		if deadlineError {
			if err != nil || result == nil || result.Status != "running" {
				t.Fatalf("deadline lost latest status: %v, %v", result, err)
			}
		} else if !errors.Is(err, backendError) {
			t.Fatalf("backend error masked: %v", err)
		}
	}
}

func TestAutotuneWaitInFlightCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		calls := 0
		_, err := waitAutotuneTask(ctx, func(requestCtx context.Context) (*vmanomaly.AutotuneTaskStatus, error) {
			calls++
			if calls == 1 {
				return &vmanomaly.AutotuneTaskStatus{Status: "running"}, nil
			}
			close(entered)
			<-requestCtx.Done()
			return nil, requestCtx.Err()
		}, time.Minute, 0)
		finished <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("poll did not start")
	}
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("parent cancellation lost: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("poll did not stop after cancellation")
	}
}
