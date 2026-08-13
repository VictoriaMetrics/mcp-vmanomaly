package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/vmanomaly"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type TimeseriesCharacteristicsArgs struct {
	Query           string   `json:"query" jsonschema:"required,description=PromQL or LogsQL query to profile on sampled time series"`
	Step            string   `json:"step,omitempty" jsonschema:"description=Query step/resolution (default: '1s'; examples: '1m' '5m' '1h'). Use the same step that will be used for autotune and the final detect-anomalies task/config."`
	Start           *float64 `json:"start,omitempty" jsonschema:"description=Optional query start timestamp as Unix seconds"`
	End             *float64 `json:"end,omitempty" jsonschema:"description=Optional query end timestamp as Unix seconds"`
	DatasourceType  string   `json:"datasource_type,omitempty" jsonschema:"enum=vm,enum=vlogs,description=Datasource type (default: 'vm')"`
	DatasourceURL   string   `json:"datasource_url,omitempty" jsonschema:"format=uri,description=Optional VictoriaMetrics or VictoriaLogs datasource URL"`
	TenantID        string   `json:"tenant_id,omitempty" jsonschema:"description=Optional tenant ID for cluster datasources"`
	PassAuthHeaders bool     `json:"pass_auth_headers,omitempty" jsonschema:"description=Forward MCP request authorization headers to datasource"`
	Timezone        string   `json:"timezone,omitempty" jsonschema:"description=IANA timezone for calendar profiling, e.g. 'Europe/Warsaw'"`
	ShortGapSteps   *int     `json:"short_gap_steps,omitempty" jsonschema:"description=Short gaps to interpolate during profiling (default: 2)"`
	Verbose         bool     `json:"verbose,omitempty" jsonschema:"description=Include expanded aggregate diagnostics without per-series identifiers. Prefer false for a compact response."`
	Limit           *int     `json:"limit,omitempty" jsonschema:"description=Sampled series cap for profiling (default: 100)"`
}

type AutotuneTaskArgs struct {
	Query                   string         `json:"query" jsonschema:"required,description=Exact PromQL or LogsQL query to sample and tune"`
	TunedClassName          string         `json:"tuned_class_name" jsonschema:"required,description=Model class or alias to tune. For UI-compatible univariate models call vmanomaly_list_models first. Outside VMUI documented multivariate aliases can be tuned even though UI discovery intentionally hides them."`
	AnomalyPercentage       *float64       `json:"anomaly_percentage,omitempty" jsonschema:"description=Expected anomaly fraction in the range [0 0.5) for unsupervised tuning (conservative MCP default: 0.02)"`
	Step                    string         `json:"step,omitempty" jsonschema:"description=Query step/resolution (default: '1s'; examples: '1m' '5m' '1h'). Use the same step from time-series characteristics and the final detect-anomalies task/config."`
	Start                   *float64       `json:"start,omitempty" jsonschema:"description=Optional query start timestamp as Unix seconds"`
	End                     *float64       `json:"end,omitempty" jsonschema:"description=Optional query end timestamp as Unix seconds"`
	DatasourceType          string         `json:"datasource_type,omitempty" jsonschema:"enum=vm,enum=vlogs,description=Datasource type (default: 'vm')"`
	DatasourceURL           string         `json:"datasource_url,omitempty" jsonschema:"format=uri,description=Optional VictoriaMetrics or VictoriaLogs datasource URL"`
	TenantID                string         `json:"tenant_id,omitempty" jsonschema:"description=Optional tenant ID for cluster datasources"`
	PassAuthHeaders         bool           `json:"pass_auth_headers,omitempty" jsonschema:"description=Forward MCP request authorization headers to datasource"`
	Timezone                string         `json:"timezone,omitempty" jsonschema:"description=IANA timezone for profile-guided calendar hints, e.g. 'Europe/Warsaw'"`
	ShortGapSteps           *int           `json:"short_gap_steps,omitempty" jsonschema:"description=Short gaps to interpolate during profiling (default: 2)"`
	Limit                   *int           `json:"limit,omitempty" jsonschema:"description=Sampled series cap for profiling and shared autotune (default: 100)"`
	UseProfileHints         *bool          `json:"use_profile_hints,omitempty" jsonschema:"description=Pass sampled time-series characteristics as search hints when the selected model search space supports them (default: true)"`
	OptimizationNTrials     *int           `json:"optimization_n_trials,omitempty" jsonschema:"description=Maximum Optuna trials. Use small values for interactive Copilot runs; MCP default is 32."`
	OptimizationTimeout     *float64       `json:"optimization_timeout,omitempty" jsonschema:"description=Optimization timeout in seconds. Use small values for interactive Copilot runs; MCP default is 8."`
	OptimizationParams      map[string]any `json:"optimization_params,omitempty" jsonschema:"description=Advanced tuning controls: n_trials timeout n_jobs n_splits train_val_ratio seed validation_scheme beta show_progress_bar gc_after_trial exact optimize_complexity. Set exact=true for online models when production uses causal exact inference. optimization_n_trials and optimization_timeout override matching keys."`
	OptimizedBusinessParams []string       `json:"optimized_business_params,omitempty" jsonschema:"description=Legacy model-local business parameters to tune: detection_direction min_dev_from_expected min_rel_dev_from_expected. For vmanomaly v1.30.2+ prefer freezing stable policies during tuning and emitting them under reader.queries.<alias> in complete configs."`
	FrozenParams            map[string]any `json:"frozen_params,omitempty" jsonschema:"description=Top-level model parameters to freeze while tuning, e.g. detection_direction='above_expected'. For vmanomaly v1.30.2+ emit stable data_range detection_direction and deviation policies under reader.queries.<alias> in complete configs. Reserved keys class and class_name are rejected. For Prophet with step < 1h, include compression={window:'1h',agg_method:'mean',adjust_boundaries:true} unless sub-hour baseline patterns are required."`
}

type AutotuneTaskIDArgs struct {
	TaskID string `json:"task_id" jsonschema:"required,description=Autotune task identifier returned by vmanomaly_create_autotune_task"`
}

const (
	defaultInteractiveAutotuneTrials  = 32
	defaultInteractiveAutotuneTimeout = 8.0
	defaultInteractiveAnomalyFraction = 0.02
)

func RegisterAnalysisTools(s *server.MCPServer, client *vmanomaly.Client) {
	characteristicsTool := mcp.NewTool(
		"vmanomaly_timeseries_characteristics",
		mcp.WithDescription("Profile a sampled set of time series returned by a query. Returns compact batch characteristics such as detected seasonalities, trend share, flatness/spikiness, coverage, and sampling stats. Use this before recommending a model from data rather than from a manual description."),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           "Profile Time Series Characteristics",
			ReadOnlyHint:    ptr(true),
			DestructiveHint: ptr(false),
			OpenWorldHint:   ptr(false),
		}),
		mcp.WithInputSchema[TimeseriesCharacteristicsArgs](),
	)
	s.AddTool(characteristicsTool, mcp.NewTypedToolHandler(handleTimeseriesCharacteristics(client)))

	createAutotuneTool := mcp.NewTool(
		"vmanomaly_create_autotune_task",
		mcp.WithDescription("Start shared unsupervised autotune on sampled query results for one requested model class. Returns a task_id immediately. Use this after choosing a model class from vmanomaly_timeseries_characteristics, then poll vmanomaly_get_autotune_task until done. Pass the same step used for profiling and the final detect-anomalies task/config. Tell the user the budget and anomaly assumption before calling; MCP defaults to optimization_timeout=8, optimization_n_trials=32, and anomaly_percentage=0.02 unless overridden."),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           "Create Shared Autotune Task",
			ReadOnlyHint:    ptr(false),
			DestructiveHint: ptr(false),
			OpenWorldHint:   ptr(false),
		}),
		mcp.WithInputSchema[AutotuneTaskArgs](),
	)
	s.AddTool(createAutotuneTool, mcp.NewTypedToolHandler(handleCreateAutotuneTask(client)))

	getAutotuneTool := mcp.NewTool(
		"vmanomaly_get_autotune_task",
		mcp.WithDescription("Get a shared autotune task. While status is running, wait briefly before polling again and never issue concurrent duplicate polls. When status is done, use result_data.data.modelConfig and related result_data fields. Treat error and canceled as terminal."),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           "Get Shared Autotune Task",
			ReadOnlyHint:    ptr(true),
			DestructiveHint: ptr(false),
			OpenWorldHint:   ptr(false),
		}),
		mcp.WithInputSchema[AutotuneTaskIDArgs](),
	)
	s.AddTool(getAutotuneTool, mcp.NewTypedToolHandler(handleGetAutotuneTask(client)))

	cancelAutotuneTool := mcp.NewTool(
		"vmanomaly_cancel_autotune_task",
		mcp.WithDescription("Request cooperative cancellation of a running shared autotune task."),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           "Cancel Shared Autotune Task",
			ReadOnlyHint:    ptr(false),
			DestructiveHint: ptr(true),
			OpenWorldHint:   ptr(false),
		}),
		mcp.WithInputSchema[AutotuneTaskIDArgs](),
	)
	s.AddTool(cancelAutotuneTool, mcp.NewTypedToolHandler(handleCancelAutotuneTask(client)))
}

func handleTimeseriesCharacteristics(client *vmanomaly.Client) func(ctx context.Context, req mcp.CallToolRequest, args TimeseriesCharacteristicsArgs) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest, args TimeseriesCharacteristicsArgs) (*mcp.CallToolResult, error) {
		query, err := requiredQuery(args.Query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		profileReq := &vmanomaly.TimeseriesCharacteristicsRequest{
			Query:           query,
			Step:            defaultString(args.Step, "1s"),
			DatasourceType:  defaultString(args.DatasourceType, "vm"),
			PassAuthHeaders: args.PassAuthHeaders,
			Verbose:         args.Verbose,
		}
		if args.Start != nil {
			profileReq.Start = args.Start
		}
		if args.End != nil {
			profileReq.End = args.End
		}
		if args.DatasourceURL != "" {
			profileReq.DatasourceURL = &args.DatasourceURL
		}
		if args.TenantID != "" {
			profileReq.TenantID = &args.TenantID
		}
		if args.Timezone != "" {
			profileReq.Timezone = &args.Timezone
		}
		if args.ShortGapSteps != nil {
			profileReq.ShortGapSteps = args.ShortGapSteps
		}
		if args.Limit != nil {
			profileReq.Limit = args.Limit
		}

		result, err := client.TimeseriesCharacteristics(ctx, profileReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to profile time series: %v", err)), nil
		}

		responseJSON, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Sampled Time Series Profile:\n\n%s", string(responseJSON))), nil
	}
}

func handleCreateAutotuneTask(client *vmanomaly.Client) func(ctx context.Context, req mcp.CallToolRequest, args AutotuneTaskArgs) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest, args AutotuneTaskArgs) (*mcp.CallToolResult, error) {
		query, err := requiredQuery(args.Query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if strings.TrimSpace(args.TunedClassName) == "" {
			return mcp.NewToolResultError("Invalid tuned_class_name: provide a supported model class or alias; use vmanomaly_list_models for UI-compatible models or documented multivariate aliases outside VMUI"), nil
		}
		anomalyPercentage := defaultFloat(args.AnomalyPercentage, defaultInteractiveAnomalyFraction)
		if anomalyPercentage < 0 || anomalyPercentage >= 0.5 {
			return mcp.NewToolResultError("Invalid anomaly_percentage: expected a value in [0, 0.5)"), nil
		}
		if err := validateFrozenParams(args.FrozenParams); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid frozen_params: %v", err)), nil
		}
		if err := validateOptimizedBusinessParams(args.OptimizedBusinessParams); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid optimized_business_params: %v", err)), nil
		}
		optimizationParams, err := buildOptimizationParams(args)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid optimization parameters: %v", err)), nil
		}
		tuneReq := &vmanomaly.AutotuneTaskRequest{
			Query:                   query,
			TunedClassName:          strings.TrimSpace(args.TunedClassName),
			AnomalyPercentage:       anomalyPercentage,
			Step:                    defaultString(args.Step, "1s"),
			DatasourceType:          defaultString(args.DatasourceType, "vm"),
			PassAuthHeaders:         args.PassAuthHeaders,
			UseProfileHints:         args.UseProfileHints,
			OptimizationParams:      optimizationParams,
			OptimizedBusinessParams: args.OptimizedBusinessParams,
			FrozenParams:            args.FrozenParams,
		}
		if args.Start != nil {
			tuneReq.Start = args.Start
		}
		if args.End != nil {
			tuneReq.End = args.End
		}
		if args.DatasourceURL != "" {
			tuneReq.DatasourceURL = &args.DatasourceURL
		}
		if args.TenantID != "" {
			tuneReq.TenantID = &args.TenantID
		}
		if args.Timezone != "" {
			tuneReq.Timezone = &args.Timezone
		}
		if args.ShortGapSteps != nil {
			tuneReq.ShortGapSteps = args.ShortGapSteps
		}
		if args.Limit != nil {
			tuneReq.Limit = args.Limit
		}

		result, err := client.CreateAutotuneTask(ctx, tuneReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(
				"Failed to create shared autotune task with budget %s: %v. Do not present profile-only settings as autotuned params; ask whether to retry with a smaller budget or sampled limit.",
				describeOptimizationBudget(optimizationParams),
				err,
			)), nil
		}

		responseJSON, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf(
			"Shared Autotune Task Created\nOptimization budget: %s\nPoll vmanomaly_get_autotune_task with the returned task_id until it reaches a terminal status.\n\n%s",
			describeOptimizationBudget(optimizationParams),
			string(responseJSON),
		)), nil
	}
}

func handleGetAutotuneTask(client *vmanomaly.Client) func(ctx context.Context, req mcp.CallToolRequest, args AutotuneTaskIDArgs) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest, args AutotuneTaskIDArgs) (*mcp.CallToolResult, error) {
		result, err := client.GetAutotuneTask(ctx, args.TaskID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get shared autotune task: %v", err)), nil
		}
		responseJSON, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Shared Autotune Task:\n\n%s", string(responseJSON))), nil
	}
}

func handleCancelAutotuneTask(client *vmanomaly.Client) func(ctx context.Context, req mcp.CallToolRequest, args AutotuneTaskIDArgs) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest, args AutotuneTaskIDArgs) (*mcp.CallToolResult, error) {
		result, err := client.CancelAutotuneTask(ctx, args.TaskID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to cancel shared autotune task: %v", err)), nil
		}
		responseJSON, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Shared Autotune Cancellation Requested:\n\n%s", string(responseJSON))), nil
	}
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defaultFloat(value *float64, fallback float64) float64 {
	if value == nil {
		return fallback
	}
	return *value
}

func requiredQuery(value string) (string, error) {
	query := strings.TrimSpace(value)
	if query == "" {
		return "", fmt.Errorf("query is required: provide the exact PromQL or LogsQL expression from the user, UI, or configured server query")
	}
	return query, nil
}

func buildOptimizationParams(args AutotuneTaskArgs) (map[string]any, error) {
	params := map[string]any{
		"n_trials": defaultInteractiveAutotuneTrials,
		"timeout":  defaultInteractiveAutotuneTimeout,
	}
	for key, value := range args.OptimizationParams {
		if key == "n_trials" && args.OptimizationNTrials != nil {
			continue
		}
		if key == "timeout" && args.OptimizationTimeout != nil {
			continue
		}
		normalized, err := normalizeOptimizationParam(key, value)
		if err != nil {
			return nil, err
		}
		params[key] = normalized
	}
	if args.OptimizationNTrials != nil {
		if *args.OptimizationNTrials <= 0 {
			return nil, fmt.Errorf("optimization_n_trials must be positive")
		}
		params["n_trials"] = *args.OptimizationNTrials
	}
	if args.OptimizationTimeout != nil {
		if !isPositiveFinite(*args.OptimizationTimeout) {
			return nil, fmt.Errorf("optimization_timeout must be a positive finite number")
		}
		params["timeout"] = *args.OptimizationTimeout
	}
	return params, nil
}

func normalizeOptimizationParam(key string, value any) (any, error) {
	switch key {
	case "n_trials", "n_splits", "seed", "n_jobs":
		normalized, ok := integerValue(value)
		if !ok {
			return nil, fmt.Errorf("%s must be an integer", key)
		}
		if key == "n_jobs" && normalized != -1 && normalized <= 0 {
			return nil, fmt.Errorf("n_jobs must be positive or -1")
		}
		if key != "seed" && key != "n_jobs" && normalized <= 0 {
			return nil, fmt.Errorf("%s must be positive", key)
		}
		return normalized, nil
	case "train_val_ratio", "timeout":
		normalized, ok := floatValue(value)
		if !ok || !isPositiveFinite(normalized) {
			return nil, fmt.Errorf("%s must be a positive finite number", key)
		}
		return normalized, nil
	case "beta":
		normalized, ok := floatValue(value)
		if !ok || !isNonNegativeFinite(normalized) {
			return nil, fmt.Errorf("beta must be a non-negative finite number")
		}
		return normalized, nil
	case "show_progress_bar", "gc_after_trial", "exact", "optimize_complexity":
		if _, ok := value.(bool); !ok {
			return nil, fmt.Errorf("%s must be a boolean", key)
		}
		return value, nil
	case "validation_scheme":
		scheme, ok := value.(string)
		if !ok || (scheme != "regular" && scheme != "leaky") {
			return nil, fmt.Errorf("validation_scheme must be regular or leaky")
		}
		return scheme, nil
	default:
		return nil, fmt.Errorf("unsupported key %q", key)
	}
}

func integerValue(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case float64:
		if !math.IsNaN(typed) && !math.IsInf(typed, 0) && typed == math.Trunc(typed) {
			return int(typed), true
		}
	}
	return 0, false
}

func floatValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case float64:
		return typed, true
	}
	return 0, false
}

func isPositiveFinite(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func isNonNegativeFinite(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func validateFrozenParams(params map[string]any) error {
	for _, key := range []string{"class", "class_name"} {
		if _, exists := params[key]; exists {
			return fmt.Errorf("reserved key %q is not allowed", key)
		}
	}
	return nil
}

func validateOptimizedBusinessParams(params []string) error {
	allowed := map[string]struct{}{
		"detection_direction":       {},
		"min_dev_from_expected":     {},
		"min_rel_dev_from_expected": {},
	}
	for _, param := range params {
		if _, exists := allowed[param]; !exists {
			return fmt.Errorf("unsupported parameter %q", param)
		}
	}
	return nil
}

func describeOptimizationBudget(params map[string]any) string {
	return fmt.Sprintf("timeout=%vs, n_trials=%v", params["timeout"], params["n_trials"])
}
