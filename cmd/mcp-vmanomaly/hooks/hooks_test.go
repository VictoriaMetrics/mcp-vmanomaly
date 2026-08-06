package hooks

import (
	"context"
	"errors"
	"testing"
)

func TestErrorClass(t *testing.T) {
	testCases := []struct {
		name string
		err  error
		want string
	}{
		{name: "none", err: nil, want: "none"},
		{name: "canceled", err: context.Canceled, want: "canceled"},
		{name: "deadline", err: context.DeadlineExceeded, want: "timeout"},
		{name: "policy", err: errors.New("requested tool is not available"), want: "policy_denied"},
		{name: "connection", err: errors.New("dial tcp: connection refused"), want: "unavailable"},
		{name: "invalid", err: errors.New("invalid request"), want: "invalid_request"},
		{name: "other", err: errors.New("secret internal detail"), want: "internal"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := ErrorClass(testCase.err); got != testCase.want {
				t.Fatalf("ErrorClass() = %q; want %q", got, testCase.want)
			}
		})
	}
}

func TestMetricLabelValueEscapesUntrustedInput(t *testing.T) {
	got := metricLabelValue("value\"}\nmalicious_metric 1")
	want := `"value\"}\nmalicious_metric 1"`
	if got != want {
		t.Fatalf("metricLabelValue() = %q; want %q", got, want)
	}
}
