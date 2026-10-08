package vmanomaly

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Capacity calls use fixed routes and the existing authenticated, timed client.
// Keep the server's evolving experimental response intact, including warnings.
func (c *Client) Capacity(ctx context.Context, operation string, payload map[string]any) (json.RawMessage, error) {
	method := http.MethodPost
	var body any
	switch operation {
	case "estimate", "throughput":
		if payload == nil {
			return nil, fmt.Errorf("capacity request must be a JSON object")
		}
		data, err := json.Marshal(payload)
		if err != nil {
			// Custom marshal errors may contain private values or paths.
			return nil, fmt.Errorf("capacity request contains values that cannot be encoded as JSON")
		}
		if len(data) > 64*1024 {
			return nil, fmt.Errorf("capacity request exceeds 64 KiB (%d bytes)", len(data))
		}
		// Send the checked snapshot without invoking custom marshalers again.
		body = json.RawMessage(data)
	case "profiles":
		method = http.MethodGet
	default:
		return nil, fmt.Errorf("unknown capacity operation")
	}
	if method == http.MethodPost {
		// Include admission wait in the existing request timeout, not just HTTP I/O.
		if timeout := c.httpClient.Timeout; timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		release, err := c.admitCapacity(ctx)
		if err != nil {
			return nil, err
		}
		defer release()
	}
	data, err := c.doRequest(ctx, method, "/api/v1/deployment-sizing/"+operation, body)
	if err != nil {
		return nil, err
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("capacity server returned invalid JSON")
	}
	return json.RawMessage(data), nil
}

// Tool calls can be parallel even within one AI response. Pace this client's
// sizing calls before dispatch; other clients can still receive HTTP 429.
func (c *Client) admitCapacity(ctx context.Context) (func(), error) {
	select {
	case c.capacityQueue <- struct{}{}:
	default:
		return nil, fmt.Errorf("capacity client queue is full; try later")
	}
	select {
	case c.capacitySlot <- struct{}{}:
	case <-ctx.Done():
		<-c.capacityQueue
		return nil, ctx.Err()
	}
	release := func() { <-c.capacitySlot; <-c.capacityQueue }
	if delay := time.Until(c.capacityNext); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			release()
			return nil, ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		release()
		return nil, err
	}
	return func() {
		c.capacityNext = time.Now().Add(100 * time.Millisecond)
		release()
	}, nil
}
