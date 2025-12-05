package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/VictoriaMetrics-Community/mcp-vmanomaly/internal/vmanomaly"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ============================================================================
// Anomaly Detection Task Tool Arguments (Struct-based schemas)
// ============================================================================

// CreateDetectionTaskArgs defines arguments for create_detection_task tool
type CreateDetectionTaskArgs struct {
	Query            string         `json:"query" jsonschema_description:"PromQL or LogsQL query defining the metrics/logs to monitor for anomalies. For metrics use PromQL syntax (e.g. 'rate(requests_total[5m])'). For logs use LogsQL. The query should return time-series data suitable for anomaly detection. Test with vmanomaly_query_metrics first."`
	ModelSpec        map[string]any `json:"model_spec" jsonschema_description:"Model configuration object specifying the anomaly detection algorithm and parameters. Must include 'class' field (e.g. 'prophet' 'zscore' 'holtwinters') plus model-specific parameters. Use vmanomaly_list_models to see available models vmanomaly_get_model_schema to see parameters and vmanomaly_validate_model_config to validate before creating task."`
	Step             string         `json:"step,omitempty" jsonschema_description:"Query resolution/step interval in Go duration format. Determines how frequently data points are sampled. Examples: '1s' '1m' '5m' '1h'. Smaller steps provide finer granularity but increase compute cost. Default: '1s'"`
	FitWindow        string         `json:"fit_window,omitempty" jsonschema_description:"Historical time window for model training in Go duration format. The model learns normal behavior patterns from this window. Examples: '1h' '6h' '1d' (1 day) '7d' (7 days) '30d'. Longer windows capture more patterns but require more data and compute. Default: '1d'"`
	FitEvery         string         `json:"fit_every,omitempty" jsonschema_description:"Model retraining frequency in Go duration format. How often the model updates its understanding of normal behavior. Examples: '1h' '6h' '1d' '7d'. More frequent retraining adapts to changes faster but uses more resources. Default: '1d'"`
	StartInferS      float64        `json:"start_infer_s,omitempty" jsonschema_description:"Inference start timestamp as Unix epoch seconds. Defines when to begin anomaly detection. Example: 1640995200 for 2022-01-01. Omit to start inference immediately from current time."`
	EndInferS        float64        `json:"end_infer_s,omitempty" jsonschema_description:"Inference end timestamp as Unix epoch seconds. Defines when to stop anomaly detection. Example: 1641081600 for 2022-01-02. Omit for continuous detection until task is canceled. Use for batch/historical analysis."`
	InferEvery       string         `json:"infer_every,omitempty" jsonschema_description:"Inference cadence for exact-mode batch processing in Go duration format. How often to run inference when exact=true. Examples: '1m' '5m' '15m' '1h'. Only relevant when exact is true. Use for scheduled periodic inference instead of continuous."`
	Exact            bool           `json:"exact,omitempty" jsonschema_description:"Enable exact-mode inference for online models. When true processes data in fixed batches at infer_every intervals. When false (default) processes data continuously in streaming mode. Set true for precise batch processing false for real-time detection. Default: false"`
	AnomalyThreshold float64        `json:"anomaly_threshold,omitempty" jsonschema_description:"Anomaly score threshold for flagging anomalies. Data points with anomaly scores above this threshold are marked as anomalous. Typical range: 0.5-3.0. Higher values = fewer false positives but may miss subtle anomalies. Lower values = more sensitive but more false positives. Default: 1.0"`
	DatasourceURL    string         `json:"datasource_url,omitempty" jsonschema_description:"Full URL of the datasource including protocol and port. Example: 'http://victoriametrics:8428' or 'http://victorialogs:9428'. If omitted defaults to VMANOMALY_ENDPOINT environment variable. Must be accessible from vmanomaly server."`
	DatasourceType   string         `json:"datasource_type,omitempty" jsonschema_description:"Type of datasource to query. Valid values: 'vm' (VictoriaMetrics for metrics) or 'vmlogs' (VictoriaLogs for logs). Default: 'vm'"`
	TenantID         string         `json:"tenant_id,omitempty" jsonschema_description:"Tenant identifier for multi-tenant VictoriaMetrics/VictoriaLogs deployments. Format: 'accountID' or 'accountID:projectID'. Omit for single-tenant setups."`
	PassAuthHeaders  bool           `json:"pass_auth_headers,omitempty" jsonschema_description:"When true forwards the Authorization header from the MCP request to the datasource. Enable this when your datasource requires authentication. Default: false"`
}

// GetTaskStatusArgs defines arguments for get_task_status tool
type GetTaskStatusArgs struct {
	TaskID string `json:"task_id" jsonschema_description:"Unique task identifier to query. This is the task_id value returned by vmanomaly_create_detection_task when the task was created. Used to monitor task progress completion status and retrieve results."`
}

// ListTasksArgs defines arguments for list_tasks tool
type ListTasksArgs struct {
	Limit  int    `json:"limit,omitempty" jsonschema_description:"Maximum number of tasks to return in the list. Range: 1-1000. Use smaller values for quick overviews larger values for comprehensive listings. Default: 20"`
	Status string `json:"status,omitempty" jsonschema_description:"Filter tasks by their current status. Valid values: 'pending' (queued not started) 'running' (actively processing) 'done' (completed successfully) 'error' (failed with error) 'canceled' (stopped by user). Omit to return tasks in all states."`
}

// CancelTaskArgs defines arguments for cancel_task tool
type CancelTaskArgs struct {
	TaskID string `json:"task_id" jsonschema_description:"Task identifier to cancel. This is the task_id returned when the task was created with vmanomaly_create_detection_task. Canceling stops task execution gracefully. Cannot cancel tasks that are already 'done' or 'error' state."`
}

// ============================================================================
// Tool Registration Functions
// ============================================================================

// RegisterTaskTools registers all anomaly detection task tools
func RegisterTaskTools(s *server.MCPServer, client *vmanomaly.Client) {
	// create_detection_task tool
	createTaskTool := mcp.NewTool(
		"vmanomaly_create_detection_task",
		mcp.WithDescription("Create and start a new anomaly detection task. The task runs asynchronously to detect anomalies in the specified query data. Returns a task_id for monitoring progress with vmanomaly_get_task_status. Before creating validate your model with vmanomaly_validate_model_config and test your query with vmanomaly_query_metrics."),
		mcp.WithInputSchema[CreateDetectionTaskArgs](),
	)
	s.AddTool(createTaskTool, mcp.NewTypedToolHandler(handleCreateDetectionTask(client)))

	// get_task_status tool
	getTaskStatusTool := mcp.NewTool(
		"vmanomaly_get_task_status",
		mcp.WithDescription("Get detailed status of a specific anomaly detection task including current state (pending running done error canceled) progress percentage timestamps metrics and results. Poll this periodically to monitor long-running tasks. When status is 'done' the response includes detected anomalies."),
		mcp.WithInputSchema[GetTaskStatusArgs](),
	)
	s.AddTool(getTaskStatusTool, mcp.NewTypedToolHandler(handleGetTaskStatus(client)))

	// list_tasks tool
	listTasksTool := mcp.NewTool(
		"vmanomaly_list_tasks",
		mcp.WithDescription("List anomaly detection tasks with optional filtering by status. Returns summary information for each task including task_id status progress and timestamps. Use this to see all active tasks find tasks by status or get an overview of system activity. Combine with vmanomaly_get_task_status for detailed task information."),
		mcp.WithInputSchema[ListTasksArgs](),
	)
	s.AddTool(listTasksTool, mcp.NewTypedToolHandler(handleListTasks(client)))

	// cancel_task tool
	cancelTaskTool := mcp.NewTool(
		"vmanomaly_cancel_task",
		mcp.WithDescription("Cancel a running or pending anomaly detection task. The task will be stopped gracefully and its status will change to 'canceled'. Use this to stop long-running tasks or tasks started by mistake. Cannot cancel tasks that are already 'done' or 'error' state. Check task status with vmanomaly_get_task_status first."),
		mcp.WithInputSchema[CancelTaskArgs](),
	)
	s.AddTool(cancelTaskTool, mcp.NewTypedToolHandler(handleCancelTask(client)))

	// get_detection_limits tool
	getLimitsTool := mcp.NewTool(
		"vmanomaly_get_detection_limits",
		mcp.WithDescription("Get current system capacity and resource limits for anomaly detection tasks. Returns maximum concurrent tasks allowed currently running task count and available slots. Use this before creating new tasks to check if system has capacity especially when running many parallel detection jobs."),
	)
	s.AddTool(getLimitsTool, handleGetDetectionLimits(client))
}

// ============================================================================
// Tool Handlers
// ============================================================================

// handleCreateDetectionTask handles the create_detection_task tool
func handleCreateDetectionTask(client *vmanomaly.Client) func(ctx context.Context, req mcp.CallToolRequest, args CreateDetectionTaskArgs) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest, args CreateDetectionTaskArgs) (*mcp.CallToolResult, error) {
		// Build request with defaults
		taskReq := &vmanomaly.AnomalyDetectionTaskRequest{
			Query:            args.Query,
			ModelSpec:        args.ModelSpec,
			Step:             "1s",
			FitWindow:        "1d",
			FitEvery:         "1d",
			Exact:            args.Exact,
			AnomalyThreshold: 1.0,
			DatasourceType:   "vm",
			PassAuthHeaders:  false,
		}

		// Override defaults if provided
		if args.Step != "" {
			taskReq.Step = args.Step
		}
		if args.FitWindow != "" {
			taskReq.FitWindow = args.FitWindow
		}
		if args.FitEvery != "" {
			taskReq.FitEvery = args.FitEvery
		}
		if args.AnomalyThreshold > 0 {
			taskReq.AnomalyThreshold = args.AnomalyThreshold
		}
		if args.DatasourceType != "" {
			taskReq.DatasourceType = args.DatasourceType
		}
		if args.StartInferS > 0 {
			taskReq.StartInferS = &args.StartInferS
		}
		if args.EndInferS > 0 {
			taskReq.EndInferS = &args.EndInferS
		}
		if args.InferEvery != "" {
			taskReq.InferEvery = &args.InferEvery
		}
		if args.TenantID != "" {
			taskReq.TenantID = &args.TenantID
		}

		// Use datasource_url from args or fall back to env var
		if args.DatasourceURL != "" {
			taskReq.DatasourceURL = &args.DatasourceURL
		} else {
			envURL := os.Getenv("VMANOMALY_ENDPOINT")
			if envURL != "" {
				taskReq.DatasourceURL = &envURL
			}
		}

		// Call API
		response, err := client.CreateDetectionTask(ctx, taskReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create detection task: %v", err)), nil
		}

		// Format response
		responseJSON, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}

		resultMsg := fmt.Sprintf("Anomaly Detection Task Created:\n\n%s\n\nUse get_task_status with task_id '%s' to monitor progress.", string(responseJSON), response.TaskID)
		return mcp.NewToolResultText(resultMsg), nil
	}
}

// handleGetTaskStatus handles the get_task_status tool
func handleGetTaskStatus(client *vmanomaly.Client) func(ctx context.Context, req mcp.CallToolRequest, args GetTaskStatusArgs) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest, args GetTaskStatusArgs) (*mcp.CallToolResult, error) {
		// Call API
		status, err := client.GetTaskStatus(ctx, args.TaskID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get task status: %v", err)), nil
		}

		// Format response
		responseJSON, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}

		// Add status indicator
		statusIcon := "⏳"
		switch status.Status {
		case "done":
			statusIcon = "✓"
		case "error":
			statusIcon = "✗"
		case "canceled":
			statusIcon = "⊘"
		}

		resultMsg := fmt.Sprintf("%s Task Status (%s):\n\n%s", statusIcon, status.Status, string(responseJSON))
		return mcp.NewToolResultText(resultMsg), nil
	}
}

// handleListTasks handles the list_tasks tool
func handleListTasks(client *vmanomaly.Client) func(ctx context.Context, req mcp.CallToolRequest, args ListTasksArgs) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest, args ListTasksArgs) (*mcp.CallToolResult, error) {
		// Set defaults
		limit := 20
		if args.Limit > 0 {
			limit = args.Limit
		}

		var statusFilter *string
		if args.Status != "" {
			statusFilter = &args.Status
		}

		// Call API
		tasks, err := client.ListTasks(ctx, limit, statusFilter)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list tasks: %v", err)), nil
		}

		// Format response
		responseJSON, err := json.MarshalIndent(tasks, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}

		resultMsg := fmt.Sprintf("Anomaly Detection Tasks (found %d):\n\n%s", len(tasks.Tasks), string(responseJSON))
		return mcp.NewToolResultText(resultMsg), nil
	}
}

// handleCancelTask handles the cancel_task tool
func handleCancelTask(client *vmanomaly.Client) func(ctx context.Context, req mcp.CallToolRequest, args CancelTaskArgs) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest, args CancelTaskArgs) (*mcp.CallToolResult, error) {
		// Call API
		result, err := client.CancelTask(ctx, args.TaskID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to cancel task: %v", err)), nil
		}

		// Format response
		responseJSON, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}

		resultMsg := fmt.Sprintf("Task Canceled:\n\n%s\n\nTask '%s' has been canceled successfully.", string(responseJSON), args.TaskID)
		return mcp.NewToolResultText(resultMsg), nil
	}
}

// handleGetDetectionLimits handles the get_detection_limits tool
func handleGetDetectionLimits(client *vmanomaly.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Call API
		limits, err := client.GetDetectionLimits(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get detection limits: %v", err)), nil
		}

		// Format response
		responseJSON, err := json.MarshalIndent(limits, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}

		// Calculate capacity percentage
		capacityPct := 0
		if limits.MaxConcurrent > 0 {
			capacityPct = (limits.Running * 100) / limits.MaxConcurrent
		}

		resultMsg := fmt.Sprintf("Anomaly Detection System Limits:\n\n%s\n\nCapacity Usage: %d%% (%d/%d tasks running)",
			string(responseJSON), capacityPct, limits.Running, limits.MaxConcurrent)
		return mcp.NewToolResultText(resultMsg), nil
	}
}
