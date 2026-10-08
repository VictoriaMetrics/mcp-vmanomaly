package tools

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/vmanomaly"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type CapacityWorkload struct {
	ModelClass        string         `json:"model_class" jsonschema_description:"Concrete built-in model; mad/mad_online and zscore/zscore_online resolve to the same online classes. Prefer mad_online or zscore_online. Autotune wrappers are unsupported. Experimental peer_outlier requires options.topology=wide and channels_per_entity=peers per pool on a supporting server."`
	EntityCount       int64          `json:"entity_count" jsonschema_description:"Active logical entities, not datapoints: series for univariate, groups for multivariate, pools for wide peer_outlier. Never substitute peer count for pool count."`
	InferEverySeconds float64        `json:"infer_every_seconds" jsonschema_description:"Seconds between inference cycles, e.g. 300 for five minutes."`
	Options           map[string]any `json:"options,omitempty" jsonschema_description:"Additional workload fields: alias, model_params, fit (window_seconds, step_seconds; omit fit_every_seconds for online models unless the user requests periodic refits; never derive it from the window; inclusive points=floor(window/step)+1), infer_points_per_cycle (defaults to ceil(cadence/step) for non-overlapping cycles; set explicitly for a different inference window), topology, channels_per_entity, query_count or retention."`
}

type CapacityEstimateArgs struct {
	Workloads []CapacityWorkload `json:"workloads" jsonschema_description:"One to sixteen model workloads. State required history and sampling when known; defaults are two weeks of five-minute data."`
	Options   map[string]any     `json:"options,omitempty" jsonschema_description:"Additional capacity request fields: policy (cpu_candidates <= max_cpus_per_node, storage_mode, persist_models, ram_limit_bytes_per_node, margin_ratio, fit_budget_seconds, infer_budget_seconds (defaults to the shortest workload cadence); storage_mode is memory or disk), deployment (members_count, replication_factor, split_by), network, disk, reader, or vmanomaly_version. Omit unspecified options rather than sending null. Cannot override workloads."`
}

type CapacityThroughputArgs struct {
	ModelClass        string         `json:"model_class" jsonschema_description:"Concrete built-in model; mad/mad_online and zscore/zscore_online resolve to the same online classes. Prefer mad_online or zscore_online."`
	CPUs              int            `json:"cpus" jsonschema_description:"CPU/worker allocation on one instance, 1 through 256."`
	InferEverySeconds float64        `json:"infer_every_seconds" jsonschema_description:"Inference cadence in seconds, e.g. 300."`
	RAMLimitBytes     *int64         `json:"ram_limit_bytes,omitempty" jsonschema_description:"Optional container RAM limit, in bytes. Omit when unconstrained."`
	Options           map[string]any `json:"options,omitempty" jsonschema_description:"Additional throughput fields: model_params, topology, channels_per_entity, history (window_seconds, step_seconds; optional fit_every_seconds; inclusive points=floor(window/step)+1), infer_points_per_cycle, storage_mode, network, disk, margin_ratio, max_models or vmanomaly_version. storage_mode accepts memory or disk (not ram). If omitted with explicit numeric history.step_seconds, MCP defaults infer_points_per_cycle to ceil(cadence/step) for non-overlapping cycles and reports that assumption; supply an explicit value for other batches. Omit unspecified options rather than sending null. Cannot override the named required fields or RAM limit."`
}

func capacityPayload(options map[string]any, required map[string]any) (map[string]any, error) {
	result := make(map[string]any, len(options)+len(required))
	for key, value := range options {
		if _, conflict := required[key]; conflict {
			return nil, fmt.Errorf("options must not override %s", key)
		}
		result[key] = value
	}
	for key, value := range required {
		if value != nil {
			result[key] = value
		}
	}
	return result, nil
}

func capacityResult(ctx context.Context, client *vmanomaly.Client, operation string, payload map[string]any) (*mcp.CallToolResult, error) {
	data, err := client.Capacity(ctx, operation, payload)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if operation == "estimate" || operation == "throughput" {
		data = capacityDisplay(data)
	}
	return mcp.NewToolResultText(string(data)), nil
}

func handleCapacityEstimate(client *vmanomaly.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args CapacityEstimateArgs
		if err := req.BindArguments(&args); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(args.Workloads) == 0 || len(args.Workloads) > 16 {
			return mcp.NewToolResultError("provide 1 to 16 workloads"), nil
		}
		workloads := make([]map[string]any, 0, len(args.Workloads))
		notes := []string{}
		deadline := math.Inf(1)
		for _, workload := range args.Workloads {
			if workload.ModelClass == "" || workload.EntityCount < 1 || workload.InferEverySeconds <= 0 {
				return mcp.NewToolResultError("each workload requires model_class, positive entity_count and infer_every_seconds"), nil
			}
			row, err := capacityPayload(workload.Options, map[string]any{"model_class": workload.ModelClass, "entity_count": workload.EntityCount, "infer_every_seconds": workload.InferEverySeconds})
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if note := defaultScoredPoints(row, "fit"); note != "" {
				notes = append(notes, fmt.Sprintf("Workload %d: %s", len(workloads)+1, note))
			}
			deadline = math.Min(deadline, workload.InferEverySeconds)
			workloads = append(workloads, row)
		}
		payload, err := capacityPayload(args.Options, map[string]any{"workloads": workloads})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		policy := map[string]any{}
		if value, exists := payload["policy"]; exists {
			original, ok := value.(map[string]any)
			if !ok {
				return mcp.NewToolResultError("options.policy must be an object"), nil
			}
			for key, value := range original {
				policy[key] = value
			}
		}
		if _, explicit := policy["infer_budget_seconds"]; !explicit {
			policy["infer_budget_seconds"] = deadline
			notes = append(notes, fmt.Sprintf("Inference deadline defaulted to the shortest workload cadence: %g seconds.", deadline))
		}
		payload["policy"] = policy
		result, err := capacityResult(ctx, client, "estimate", payload)
		if err == nil && len(notes) > 0 {
			result.Content = append(result.Content, mcp.NewTextContent(strings.Join(notes, " ")))
		}
		return result, err
	}
}

func handleCapacityThroughput(client *vmanomaly.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args CapacityThroughputArgs
		if err := req.BindArguments(&args); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if args.ModelClass == "" || args.CPUs < 1 || args.CPUs > 256 || args.InferEverySeconds <= 0 || (args.RAMLimitBytes != nil && *args.RAMLimitBytes <= 0) {
			return mcp.NewToolResultError("provide model_class, 1–256 cpus, positive infer_every_seconds and an optional positive RAM limit"), nil
		}
		// Reserve the optional named field even when absent, so options cannot override it.
		required := map[string]any{"model_class": args.ModelClass, "cpus": args.CPUs, "infer_every_seconds": args.InferEverySeconds, "ram_limit_bytes": nil}
		if args.RAMLimitBytes != nil {
			required["ram_limit_bytes"] = *args.RAMLimitBytes
		}
		payload, err := capacityPayload(args.Options, required)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		note := defaultScoredPoints(payload, "history")
		result, err := capacityResult(ctx, client, "throughput", payload)
		if err == nil && note != "" {
			result.Content = append(result.Content, mcp.NewTextContent(note))
		}
		return result, err
	}
}

// When sampling is explicit, score the new samples arriving in one cycle unless
// the caller supplies an inference batch. Keep backend validation authoritative.
func defaultScoredPoints(payload map[string]any, historyKey string) string {
	if _, explicit := payload["infer_points_per_cycle"]; explicit {
		return ""
	}
	history, ok := payload[historyKey].(map[string]any)
	if !ok {
		return ""
	}
	step, ok := history["step_seconds"].(float64)
	cadence, cadenceOK := payload["infer_every_seconds"].(float64)
	if !ok || !cadenceOK || step <= 0 || cadence <= 0 {
		return ""
	}
	points := math.Ceil(cadence / step)
	if math.IsNaN(points) || math.IsInf(points, 0) {
		return ""
	}
	payload["infer_points_per_cycle"] = points
	return fmt.Sprintf("Assumed non-overlapping cycles: infer_points_per_cycle=%.0f from ceil(infer_every_seconds/%s.step_seconds). Supply an explicit count for another inference window.", points, historyKey)
}

func RegisterCapacityTools(s *server.MCPServer, client *vmanomaly.Client) {
	peerNote := " Experimental peer_outlier sizing on supporting servers requires options.topology=wide and options.channels_per_entity=peers per pool (3–10,000, meeting min_peer_count, default 5). Entities/models count pools; outputs and input_series count peer series separately. Only equal-size fixed pools with complete observations, one query per workload and no churn retention are supported. Omit model_params.groupby: pools are already grouped. Use separate forward workloads for different queries or pool sizes, never average unequal widths. Reverse sizing returns pool-model capacity. Widths above 64 are estimated from a bounded sample, preserving the requested width, with n log(n) CPU and linear memory/output scaling. Worker scaling uses shared fallback coefficients, not measured peer-specific speedup. Never reduce a 100-peer request to 64 peers; send its actual width. Preserve operational limits, but do not explain calibration mechanics in normal replies. For peer groups use the returned short estimate_notice once; no extrapolation formulas or coefficients. Shipped profile absence does not preclude bounded live calibration on the installed server version; report unsupported-server errors without falling back to univariate sizing."
	versionNote := " MCMC-based sizing is unsupported; do not retry it with alternate parameters. Targets before 1.31.0 are unsupported. Omitted version uses the server version; newer targets inherit the latest earlier profile. Preserve a supplied target and report inherited estimates without claiming exact-version validation."
	parameterNote := " Check vmanomaly_get_model_schema or a matching sizing profile for required model parameters before calling. Ask for missing choices; do not invent them. Report actionable 422 details without internal execution mechanics. An unsupported configuration requires a matching profile or offline calibration, not fewer series or a different cadence."
	diskNote := " Omitted disk bandwidth means no extra throughput adjustment; measured persistence/filesystem costs remain embedded in profiles."
	parameterNote += peerNote
	annotation := mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: ptr(true), DestructiveHint: ptr(false), OpenWorldHint: ptr(false)})
	s.AddTool(mcp.NewTool("vmanomaly_estimate_deployment_resources",
		mcp.WithDescription("Experimental vmanomaly deployment resource sizing, not monitored-metric forecasting or forecast_at. FORWARD sizing for a known entity_count: estimate required CPU, RAM, disk and fit/inference stage costs. When the user instead gives CPU/RAM and cadence without a series count, use vmanomaly_estimate_inference_capacity; do not ask for cardinality to make a forward request. For suspected OOM distinguish observed events from CrashLoopBackOff. For an existing single pod set options.deployment.members_count=1. Online models default to bootstrap only and advisory fit duration, retaining peak RAM/disk. Omit fit_every_seconds unless the user explicitly requests periodic refits. fit_window is history depth, NEVER a refit cadence; do not copy it into fit_every_seconds or reuse an assistant-invented refit interval. mad and mad_online are the same online model (likewise zscore/zscore_online). For requested query sharding use total entity_count, workload options.query_count and options.deployment with members_count and split_by=queries. If Q equal query shards are proposed without a node count, start with Q members and disclose one member per shard. Preserve that deployment after errors. Automatic plans are hypothetical repartitions; extra nodes do not split fixed queries. Evaluate CPU candidates in ONE call; set both cpu_candidates and max_cpus_per_node for larger CPUs. On deployment_infeasible, report the binding constraints and STOP sizing calls for this turn. Do not retry larger CPU lists or change query_count, members_count, deadlines or workload size to find an alternative without user approval. Report that only the submitted CPU candidates were checked. Preserve history and headroom. Use one short experimental-estimate caveat. Do not display extrapolation factors, benchmark sample sizes or internal calibration mechanics. Use resolved_request to confirm scored points and deadlines. total_seconds is nominal; planned_seconds includes headroom, not a range. Never claim a cadence fits when planned time exceeds it. Online bootstrap is advisory; network covers writers only. storage_mode is an estimator option, not deployment YAML. Bounded calibration may run; never changes deployment."+diskNote+versionNote+parameterNote),
		annotation, mcp.WithInputSchema[CapacityEstimateArgs]()), handleCapacityEstimate(client))
	s.AddTool(mcp.NewTool("vmanomaly_estimate_inference_capacity",
		mcp.WithDescription("Experimental vmanomaly deployment inference capacity, not monitored-metric forecasting or forecast_at. REVERSE sizing: given CPUs, optional RAM limit and inference cadence, return approximate maximum active models per interval. Choose this for 'estimate capacity' with fixed resources and no series count. Active model/entity count is the OUTPUT: never request it as an input or invent it. For univariate models this is series count; multivariate entities are groups with channels_per_entity input series each. Report input_series separately. Clarify missing history or disclose a default; retain the supplied version without reconfirmation. To compare RAM vs disk, call sequentially with options.storage_mode=memory and disk, keeping other inputs fixed. Inference-only: excludes refits/bootstrap/churn. Preserve limiting constraints; use one short experimental-estimate caveat, without extrapolation factors, benchmark sample sizes or internal calibration mechanics. Not a validated maximum. Bounded calibration may run; never changes deployment."+diskNote+versionNote+parameterNote),
		annotation, mcp.WithInputSchema[CapacityThroughputArgs]()), handleCapacityThroughput(client))
	s.AddTool(mcp.NewTool("vmanomaly_get_deployment_sizing_profiles",
		mcp.WithDescription("List shipped experimental deployment-sizing model/parameter profiles, target versions and CPU reference. Use to discover calibrated configurations before sizing; unavailable on older servers."), annotation),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return capacityResult(ctx, client, "profiles", nil)
		})
}
