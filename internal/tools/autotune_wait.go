package tools

import (
	"context"
	"time"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/vmanomaly"
)

// Interactive latency bounds: at most ten seconds per tool call, with two-second
// spacing to avoid busy polling. These are responsiveness choices, not model limits.
const maxAutotuneWaitSeconds = 10
const autotunePollInterval = 2 * time.Second

func waitAutotuneTask(ctx context.Context, get func(context.Context) (*vmanomaly.AutotuneTaskStatus, error), wait, interval time.Duration) (*vmanomaly.AutotuneTaskStatus, error) {
	if wait == 0 {
		return get(ctx)
	}
	waitCtx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	var latest *vmanomaly.AutotuneTaskStatus
	for {
		result, err := get(waitCtx)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err != nil {
			if waitCtx.Err() != nil && latest != nil {
				return latest, nil
			}
			return nil, err
		}
		latest = result
		if result.Status != "running" && result.Status != "pending" && result.Status != "queued" {
			return result, nil
		}
		timer := time.NewTimer(interval)
		select {
		case <-waitCtx.Done():
			timer.Stop()
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return latest, nil
		case <-timer.C:
		}
	}
}
