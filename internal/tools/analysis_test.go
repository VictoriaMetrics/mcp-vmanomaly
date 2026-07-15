package tools

import "testing"

func TestBuildOptimizationParamsUsesInteractiveDefaults(t *testing.T) {
	params, err := buildOptimizationParams(AutotuneTaskArgs{})
	if err != nil {
		t.Fatalf("buildOptimizationParams() error = %v", err)
	}

	if params["n_trials"] != defaultInteractiveAutotuneTrials {
		t.Fatalf("n_trials = %v, want %v", params["n_trials"], defaultInteractiveAutotuneTrials)
	}
	if params["timeout"] != defaultInteractiveAutotuneTimeout {
		t.Fatalf("timeout = %v, want %v", params["timeout"], defaultInteractiveAutotuneTimeout)
	}
}

func TestInteractiveAnomalyFractionMatchesGuidance(t *testing.T) {
	if defaultInteractiveAnomalyFraction != 0.02 {
		t.Fatalf("default anomaly fraction = %v, want 0.02", defaultInteractiveAnomalyFraction)
	}
}

func TestBuildOptimizationParamsLetsExplicitFieldsOverrideMap(t *testing.T) {
	nTrials := 5
	timeout := 2.5
	params, err := buildOptimizationParams(AutotuneTaskArgs{
		OptimizationNTrials: &nTrials,
		OptimizationTimeout: &timeout,
		OptimizationParams: map[string]any{
			"n_trials": "ignored because the explicit field wins",
			"timeout":  "ignored because the explicit field wins",
			"n_splits": float64(2),
		},
	})
	if err != nil {
		t.Fatalf("buildOptimizationParams() error = %v", err)
	}

	if params["n_trials"] != nTrials {
		t.Fatalf("n_trials = %v, want %v", params["n_trials"], nTrials)
	}
	if params["timeout"] != timeout {
		t.Fatalf("timeout = %v, want %v", params["timeout"], timeout)
	}
	if params["n_splits"] != 2 {
		t.Fatalf("n_splits = %v, want 2", params["n_splits"])
	}
}

func TestBuildOptimizationParamsNormalizesIntegralJSONNumbers(t *testing.T) {
	params, err := buildOptimizationParams(AutotuneTaskArgs{
		OptimizationParams: map[string]any{
			"n_splits": float64(2),
			"seed":     float64(42),
		},
	})
	if err != nil {
		t.Fatalf("buildOptimizationParams() error = %v", err)
	}

	for _, key := range []string{"n_splits", "seed"} {
		if _, ok := params[key].(int); !ok {
			t.Fatalf("%s type = %T, want int", key, params[key])
		}
	}
}

func TestBuildOptimizationParamsAcceptsNumericRatioAndZeroBeta(t *testing.T) {
	params, err := buildOptimizationParams(AutotuneTaskArgs{
		OptimizationParams: map[string]any{
			"train_val_ratio": 2.5,
			"beta":            0,
		},
	})
	if err != nil {
		t.Fatalf("buildOptimizationParams() error = %v", err)
	}
	if params["train_val_ratio"] != 2.5 {
		t.Fatalf("train_val_ratio = %v, want 2.5", params["train_val_ratio"])
	}
	if params["beta"] != float64(0) {
		t.Fatalf("beta = %v, want 0", params["beta"])
	}
}

func TestBuildOptimizationParamsAcceptsOnlineValidationControls(t *testing.T) {
	params, err := buildOptimizationParams(AutotuneTaskArgs{
		OptimizationParams: map[string]any{
			"exact":               true,
			"optimize_complexity": true,
		},
	})
	if err != nil {
		t.Fatalf("buildOptimizationParams() error = %v", err)
	}
	for _, key := range []string{"exact", "optimize_complexity"} {
		if params[key] != true {
			t.Fatalf("%s = %v, want true", key, params[key])
		}
	}
}

func TestBuildOptimizationParamsRejectsUnsupportedOrInvalidValues(t *testing.T) {
	tests := []AutotuneTaskArgs{
		{OptimizationParams: map[string]any{"unknown": 1}},
		{OptimizationParams: map[string]any{"n_splits": 2.5}},
		{OptimizationParams: map[string]any{"train_val_ratio": 0}},
		{OptimizationParams: map[string]any{"beta": -0.1}},
		{OptimizationParams: map[string]any{"timeout": 0}},
	}

	for _, args := range tests {
		if _, err := buildOptimizationParams(args); err == nil {
			t.Fatalf("buildOptimizationParams(%v) error = nil, want error", args.OptimizationParams)
		}
	}
}

func TestValidateFrozenParamsRejectsReservedModelIdentity(t *testing.T) {
	for _, key := range []string{"class", "class_name"} {
		if err := validateFrozenParams(map[string]any{key: "mad_online"}); err == nil {
			t.Fatalf("validateFrozenParams(%q) error = nil, want error", key)
		}
	}

	if err := validateFrozenParams(map[string]any{"detection_direction": "above_expected"}); err != nil {
		t.Fatalf("validateFrozenParams() unexpected error = %v", err)
	}
}

func TestValidateOptimizedBusinessParams(t *testing.T) {
	valid := []string{"detection_direction", "min_dev_from_expected", "min_rel_dev_from_expected"}
	if err := validateOptimizedBusinessParams(valid); err != nil {
		t.Fatalf("validateOptimizedBusinessParams() unexpected error = %v", err)
	}
	if err := validateOptimizedBusinessParams([]string{"min_percentile_threshold"}); err == nil {
		t.Fatal("validateOptimizedBusinessParams() error = nil, want error")
	}
}

func TestDescribeOptimizationBudget(t *testing.T) {
	got := describeOptimizationBudget(map[string]any{
		"timeout":  8,
		"n_trials": 32,
	})

	if got != "timeout=8s, n_trials=32" {
		t.Fatalf("budget = %q", got)
	}
}

func TestRequiredQuery(t *testing.T) {
	if _, err := requiredQuery(" \n\t "); err == nil {
		t.Fatal("requiredQuery() error = nil, want error")
	}
	query, err := requiredQuery("  up{job=\"api\"}  ")
	if err != nil {
		t.Fatalf("requiredQuery() unexpected error = %v", err)
	}
	if query != `up{job="api"}` {
		t.Fatalf("query = %q", query)
	}
}
