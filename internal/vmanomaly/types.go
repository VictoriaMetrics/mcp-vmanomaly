package vmanomaly

// ============================================================================
// Anomaly Detection Task Types
// ============================================================================

// AnomalyDetectionTaskRequest represents a request to create an anomaly detection task
type AnomalyDetectionTaskRequest struct {
	Query            string         `json:"query"`                    // PromQL query to process
	StartInferS      *float64       `json:"start_infer_s,omitempty"`  // Inference start timestamp (Unix)
	EndInferS        *float64       `json:"end_infer_s,omitempty"`    // Inference end timestamp (Unix)
	Step             string         `json:"step"`                     // Query step/resolution (default: "1s")
	FitWindow        string         `json:"fit_window"`               // Time window for model fitting (default: "1d")
	FitEvery         string         `json:"fit_every"`                // Model retraining frequency (default: "1d")
	InferEvery       *string        `json:"infer_every,omitempty"`    // Optional inference cadence for exact-mode batches
	Exact            bool           `json:"exact"`                    // Enable exact-mode inference for online models
	AnomalyThreshold float64        `json:"anomaly_threshold"`        // Anomaly detection threshold (default: 1)
	ModelSpec        map[string]any `json:"model_spec,omitempty"`     // Model specification (Pydantic discriminated union)
	DatasourceURL    *string        `json:"datasource_url,omitempty"` // Datasource URL
	DatasourceType   string         `json:"datasource_type"`          // Datasource type: vm or vmlogs (default: "vm")
	TenantID         *string        `json:"tenant_id,omitempty"`      // Optional tenant ID
	PassAuthHeaders  bool           `json:"pass_auth_headers"`        // Forward Authorization header to datasource
}

// AnomalyDetectionTaskResponse represents the response after creating a task
type AnomalyDetectionTaskResponse struct {
	TaskID string `json:"task_id"` // Unique task identifier
	Status string `json:"status"`  // Task status: running|done|error|canceled
}

// AnomalyDetectionTaskStatus represents full task status information
type AnomalyDetectionTaskStatus struct {
	TaskID     string         `json:"task_id"`         // Unique task identifier
	Status     string         `json:"status"`          // Task status: running|done|error|canceled
	Progress   int            `json:"progress"`        // Progress percentage (0-100)
	Message    string         `json:"message"`         // Current status message
	StartedAt  *string        `json:"started_at"`      // Task start time (ISO format)
	UpdatedAt  string         `json:"updated_at"`      // Last update time (ISO format)
	Metrics    map[string]any `json:"metrics"`         // Task metrics and counters
	ResultData *TaskResult    `json:"result_data"`     // Task result (when done)
	Error      *string        `json:"error,omitempty"` // Error message (when failed)
}

// TaskResult represents the result from an anomaly detection task
type TaskResult struct {
	Status string         `json:"status"`          // Result status: success|error
	Data   map[string]any `json:"data,omitempty"`  // Result data when successful
	Stats  map[string]any `json:"stats,omitempty"` // Statistics
	Error  *string        `json:"error,omitempty"` // Error message when failed
}

// AnomalyDetectionTaskListItem represents a task in the list response
type AnomalyDetectionTaskListItem struct {
	TaskID    string         `json:"task_id"`    // Unique task identifier
	Status    string         `json:"status"`     // Task status
	Progress  int            `json:"progress"`   // Progress percentage
	Message   string         `json:"message"`    // Current status message
	StartedAt *string        `json:"started_at"` // Task start time
	UpdatedAt string         `json:"updated_at"` // Last update time
	Metrics   map[string]any `json:"metrics"`    // Task metrics
}

// AnomalyDetectionTaskListResponse represents the response for task listing
type AnomalyDetectionTaskListResponse struct {
	Tasks []AnomalyDetectionTaskListItem `json:"tasks"` // List of tasks
}

// AnomalyDetectionLimitsResponse represents system capacity and limits
type AnomalyDetectionLimitsResponse struct {
	MaxConcurrent int `json:"max_concurrent"` // Maximum concurrent tasks
	Running       int `json:"running"`        // Currently running tasks
	Available     int `json:"available"`      // Available task slots
}

// ============================================================================
// Model Configuration Types
// ============================================================================

// ModelValidationResponse represents the response from model validation
type ModelValidationResponse struct {
	Valid     bool           `json:"valid"`      // Whether the model settings are valid
	ModelSpec map[string]any `json:"model_spec"` // Validated full model configuration
}

// ServerModelsResponse represents configured runtime models on the vmanomaly server.
type ServerModelsResponse struct {
	Models map[string]ServerModelResponse `json:"models"` // Models keyed by configured alias
}

// ServerModelResponse represents a configured runtime model.
type ServerModelResponse struct {
	ModelConfiguration map[string]any               `json:"model_configuration"` // Model configuration object
	Queries            map[string]ServerQueryConfig `json:"queries"`             // Query configs keyed by alias
	IsOnline           bool                         `json:"is_online"`           // Whether the model is online
	IsMultivariate     bool                         `json:"is_multivariate"`     // Whether the model is multivariate
	IsUISelectable     bool                         `json:"is_ui_selectable"`    // Whether the model can be selected in UI
}

// ServerModelQueriesResponse represents full query configurations keyed by query alias.
type ServerModelQueriesResponse map[string]ServerQueryConfig

// ServerQueryConfig represents a query attached to a configured server model.
type ServerQueryConfig struct {
	Expr              string  `json:"expr"`                           // Query expression
	TZ                string  `json:"tz"`                             // Query timezone
	TenantID          *string `json:"tenant_id,omitempty"`            // Optional tenant ID
	Offset            string  `json:"offset"`                         // Query offset
	Step              string  `json:"step"`                           // Query step
	MaxPointsPerQuery *int    `json:"max_points_per_query,omitempty"` // Optional maximum points per query
}

// ModelClassEnum represents available model types
type ModelClassEnum string

const (
	ModelClassRollingQuantile     ModelClassEnum = "rolling_quantile"
	ModelClassStd                 ModelClassEnum = "std"
	ModelClassQuantileOnline      ModelClassEnum = "quantile_online"
	ModelClassZScoreOnline        ModelClassEnum = "zscore_online"
	ModelClassHoltWinters         ModelClassEnum = "holtwinters"
	ModelClassMADOnline           ModelClassEnum = "mad_online"
	ModelClassProphet             ModelClassEnum = "prophet"
	ModelClassMAD                 ModelClassEnum = "mad"
	ModelClassIsolationForestUniv ModelClassEnum = "isolation_forest_univariate"
	ModelClassZScore              ModelClassEnum = "zscore"
	ModelClassAuto                ModelClassEnum = "auto"
)

// ============================================================================
// Query Types
// ============================================================================

// QueryRequest represents a query to VictoriaMetrics/VictoriaLogs
type QueryRequest struct {
	Query           string   `json:"query"`                    // PromQL query
	Start           *float64 `json:"start,omitempty"`          // Query start timestamp (Unix)
	End             *float64 `json:"end,omitempty"`            // Query end timestamp (Unix)
	Step            string   `json:"step"`                     // Query step/resolution (default: "1s")
	DatasourceType  string   `json:"datasource_type"`          // Datasource type: vm or vmlogs (default: "vm")
	DatasourceURL   *string  `json:"datasource_url,omitempty"` // Datasource URL
	TenantID        *string  `json:"tenant_id,omitempty"`      // Optional tenant ID
	NoCache         *string  `json:"nocache,omitempty"`        // Cache bypass parameter
	PassAuthHeaders bool     `json:"pass_auth_headers"`        // Forward Authorization header
}

// ============================================================================
// Config Generation Types
// ============================================================================

// ConfigGenerationRequest represents parameters for generating a config
type ConfigGenerationRequest struct {
	Step          string         `json:"step"`                  // Query step/resolution
	Query         string         `json:"query"`                 // PromQL query
	DatasourceURL string         `json:"datasource_url"`        // Datasource URL
	TenantID      *string        `json:"tenant_id,omitempty"`   // Optional tenant ID
	FitWindow     string         `json:"fit_window"`            // Time window for model fitting (default: "1d")
	FitEvery      string         `json:"fit_every"`             // Model retraining frequency (default: "1d")
	InferEvery    *string        `json:"infer_every,omitempty"` // Optional inference cadence
	ModelSpec     map[string]any `json:"model_spec"`            // Model specification (JSON-encoded)
}

// ============================================================================
// General Response Types
// ============================================================================

// HealthResponse represents health check response
type HealthResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// ModelsListResponse represents the list of available models
type ModelsListResponse struct {
	Models []string `json:"models"`
}

type BuildInfo struct {
	Vmanomaly string `json:"vmanomaly"`
	Vmui      string `json:"vmui"`
}

type OkValidationResponse struct {
	IsValid   bool           `json:"is_valid"`
	Validated map[string]any `json:"validated"`
}

// ============================================================================
// Alert Rule Types
// ============================================================================

// AlertRuleRequest represents parameters for generating VMAlert rules
type AlertRuleRequest struct {
	Step             string   `json:"step"`                        // Query step/resolution (required)
	Query            string   `json:"query"`                       // PromQL query for alert description (required)
	AnomalyThreshold *float64 `json:"anomaly_threshold,omitempty"` // Threshold (default: 1.0)
	RuleName         *string  `json:"rule_name,omitempty"`         // Custom rule name
	GroupName        *string  `json:"group_name,omitempty"`        // VMAlert group name (default: "VMAnomalyAlerts")
	RuleDescription  *string  `json:"rule_description,omitempty"`  // Custom description
	InferEvery       *string  `json:"infer_every,omitempty"`       // Inference cadence
}

// ============================================================================
// Compatibility Check Types
// ============================================================================

type CompatibilityRequirement struct {
	RuntimeVersion  string  `json:"runtime_version"`             // Runtime version being used
	OriginVersion   string  `json:"origin_version"`              // Original version of the state
	MinStateVersion string  `json:"min_state_version"`           // Minimum compatible state version
	MaxStateVersion *string `json:"max_state_version,omitempty"` // Maximum compatible state version (if any)
	Description     string  `json:"description"`                 // Human-readable requirement description
}

type ComponentCompatibilityIssue struct {
	Component        string                   `json:"component"`                   // Component type: 'model' or 'data'
	Subcomponent     string                   `json:"subcomponent"`                // Specific subcomponent (e.g., 'prophet', 'zscore', 'vm')
	Requirement      CompatibilityRequirement `json:"requirement"`                 // Version requirement causing the issue
	Reason           *string                  `json:"reason,omitempty"`            // Explanation of incompatibility
	AffectedEntities []string                 `json:"affected_entities,omitempty"` // Affected model aliases or data sources
}

type ComponentCompatibilityAssessment struct {
	Issues                []ComponentCompatibilityIssue `json:"issues"`                   // List of component compatibility issues
	ModelsToPurge         []string                      `json:"models_to_purge"`          // Model aliases that need purging
	ShouldPurgeReaderData bool                          `json:"should_purge_reader_data"` // Whether reader data needs purging
}

type GlobalCompatibilityCheck struct {
	HasState       bool                      `json:"has_state"`             // Whether persisted state exists
	IsCompatible   bool                      `json:"is_compatible"`         // Whether state is compatible with runtime version
	Reason         *string                   `json:"reason,omitempty"`      // Explanation if incompatible or additional context
	Requirement    *CompatibilityRequirement `json:"requirement,omitempty"` // Version requirements for compatibility (if state exists)
	DropEverything bool                      `json:"drop_everything"`       // Whether all persisted state must be dropped
}

type CompatibilityCheckResponse struct {
	RuntimeVersion      string                            `json:"runtime_version"`                // Runtime version being checked against
	StoredVersion       *string                           `json:"stored_version,omitempty"`       // Version of stored state (if any)
	GlobalCheck         GlobalCompatibilityCheck          `json:"global_check"`                   // Global compatibility status
	ComponentAssessment *ComponentCompatibilityAssessment `json:"component_assessment,omitempty"` // Component-level issues (when partially compatible)
}
