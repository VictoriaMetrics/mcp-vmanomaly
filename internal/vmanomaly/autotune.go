package vmanomaly

import (
	"fmt"
	"math"
	"strings"
)

// validateAutotuneReplay checks the shared client boundary without duplicating
// the backend duration parser (which supports units beyond Go time.Duration).
func validateAutotuneReplay(req *AutotuneTaskRequest) error {
	if req.Replay == nil {
		return nil
	}
	if req.Start == nil || req.End == nil || math.IsNaN(*req.Start) || math.IsNaN(*req.End) || math.IsInf(*req.Start, 0) || math.IsInf(*req.End, 0) || *req.Start >= *req.End {
		return fmt.Errorf("replay requires finite explicit start and end timestamps with start < end")
	}
	if strings.TrimSpace(req.Replay.FitWindow) == "" || strings.TrimSpace(req.Replay.FitEvery) == "" || strings.TrimSpace(req.Replay.InferEvery) == "" {
		return fmt.Errorf("replay requires non-empty fit_window, fit_every and infer_every durations")
	}
	return nil
}
