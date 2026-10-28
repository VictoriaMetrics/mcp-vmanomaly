package vmanomaly

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// ErrInvalidAutotuneReplay identifies local replay input errors, not task budget failures.
var ErrInvalidAutotuneReplay = errors.New("invalid autotune replay")

// validateAutotuneReplay checks the shared client boundary without duplicating
// the backend duration parser (which supports units beyond Go time.Duration).
func validateAutotuneReplay(req *AutotuneTaskRequest) error {
	if req == nil || req.Replay == nil {
		return nil
	}
	if req.Start == nil || req.End == nil || math.IsNaN(*req.Start) || math.IsNaN(*req.End) || math.IsInf(*req.Start, 0) || math.IsInf(*req.End, 0) || *req.Start >= *req.End {
		return fmt.Errorf("%w: requires finite explicit start and end timestamps with start < end", ErrInvalidAutotuneReplay)
	}
	if strings.TrimSpace(req.Replay.FitWindow) == "" || strings.TrimSpace(req.Replay.FitEvery) == "" || strings.TrimSpace(req.Replay.InferEvery) == "" {
		return fmt.Errorf("%w: requires non-empty fit_window, fit_every and infer_every durations", ErrInvalidAutotuneReplay)
	}
	return nil
}
