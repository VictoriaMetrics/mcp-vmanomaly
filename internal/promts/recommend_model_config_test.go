package prompts

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestPromptConfigRecommendationIncludesDatasourceRuntimeContext(t *testing.T) {
	req := mcp.GetPromptRequest{}
	req.Params.Arguments = map[string]string{
		"query":             "up",
		"step":              "5m",
		"datasource_url":    "http://victoriametrics:8428",
		"tenant_id":         "7:3",
		"pass_auth_headers": "true",
	}

	result, err := promptConfigRecommendationHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("promptConfigRecommendationHandler() error = %v", err)
	}
	last := result.Messages[len(result.Messages)-1].Content.(mcp.TextContent).Text
	for _, expected := range []string{
		"**Query**: up",
		"**Step**: 5m",
		"**Datasource URL**: http://victoriametrics:8428",
		"**Tenant ID**: 7:3",
		"**Pass Auth Headers**: true",
	} {
		if !strings.Contains(last, expected) {
			t.Errorf("generated request does not contain %q", expected)
		}
	}
}

func TestRecommendationPromptMatchesUIProvideSeriesBoundary(t *testing.T) {
	if !strings.Contains(contextMessage, "UI model suggestions cannot modify provide_series") {
		t.Fatal("context prompt does not describe the UI provide_series boundary")
	}
	if strings.Contains(contextMessage, "unless the user explicitly asks to change output columns") {
		t.Fatal("context prompt still promises an unsupported provide_series escape hatch")
	}
}

func TestRecommendationPromptUsesProfileComplexityDefaults(t *testing.T) {
	for _, expected := range []string{
		"prefer temporal_envelope as the best balance",
		"prefer mad_online/mad when robustness is important",
		"Prefer zscore_online/zscore only when the sample is stable/light-tailed",
		"Do not recommend them for new configurations",
		"offer Temporal Envelope as the univariate or multivariate migration target",
	} {
		if !strings.Contains(contextMessage, expected) {
			t.Errorf("context prompt does not contain %q", expected)
		}
	}
}

func TestRecommendationToolGuidanceDoesNotRecommendOfflineModelsForNewConfigs(t *testing.T) {
	for _, expected := range []string{
		"Do not recommend Prophet, Holt-Winters, or Isolation Forest for new configurations",
		"offer Temporal Envelope as the univariate or multivariate migration target",
	} {
		if !strings.Contains(toolGuidanceMessage, expected) {
			t.Errorf("tool guidance does not contain %q", expected)
		}
	}
}

func TestRecommendationPromptExplainsMultivariateUIBoundary(t *testing.T) {
	for _, expected := range []string{
		"check vmanomaly_list_models and vmanomaly_get_model_schema for multivariate availability",
		"Preserve the experimental label",
		"Copy expected_revision from the current query revision",
		"preserve untouched and disabled rows",
	} {
		if !strings.Contains(contextMessage, expected) {
			t.Errorf("context prompt does not contain %q", expected)
		}
	}
	if strings.Contains(contextMessage, "arbitrary regressors") || strings.Contains(contextMessage, "custom regressors") {
		t.Fatal("context prompt claims unsupported Prophet regressors")
	}
}

func TestRecommendationToolGuidanceDescribesAggregateVerboseOutput(t *testing.T) {
	if !strings.Contains(toolGuidanceMessage, "verbose does not return per-series identifiers") {
		t.Fatal("tool guidance does not describe aggregate-only verbose output")
	}
}

func TestRecommendationPromptRequiresAndReusesExactQuery(t *testing.T) {
	for _, expected := range []string{
		"If the active model/UI query is empty and no exact query exists",
		"If the UI query input is empty but the user already supplied an exact query, do not ask again",
		"propose placing the same query in the UI query input",
		"Prefer an effective online model whenever it represents the measured profile",
		"Set optimization_params.exact=true for an online model",
		"Do not issue concurrent duplicate status calls",
	} {
		if !strings.Contains(toolGuidanceMessage, expected) {
			t.Errorf("tool guidance does not contain %q", expected)
		}
	}
}

func TestRecommendationPromptUsesQueryLevelBusinessPolicies(t *testing.T) {
	for _, expected := range []string{
		"per-query detection_direction, data_range, min_dev_from_expected and min_rel_dev_from_expected",
		"When the tool exposes queries and expected_revision",
		"complete vmanomaly v1.30.2+ deployment configurations outside that UI flow",
		"stable KPI policies belong to reader.queries.<alias>",
		"An explicit query value is authoritative",
		"move those stable policies to reader.queries.<alias>",
		"Model-level placement of those four policies is deprecated",
	} {
		if !strings.Contains(contextMessage+toolGuidanceMessage, expected) {
			t.Errorf("recommendation guidance does not contain %q", expected)
		}
	}
}

func TestRecommendationPromptUsesBoundedReaderConcurrency(t *testing.T) {
	for _, expected := range []string{
		"reader.workers bounds concurrent datasource requests",
		"Prefer workers: 0 for the automatic bound",
		"Do not confuse reader.workers with settings.n_workers",
		"VMUI Copilot has no reader concurrency suggestion field",
	} {
		if !strings.Contains(contextMessage, expected) {
			t.Errorf("context prompt does not contain %q", expected)
		}
	}
}

func TestRecommendationPromptUsesBoundedWriterAndStableShardingGuidance(t *testing.T) {
	for _, expected := range []string{
		"remains many-to-one when it emits per-channel y, forecast, or bound diagnostics",
		"writer.batch_max_series and writer.batch_max_bytes",
		"writer.metric_prefix_cache_max_entries",
		"VMANOMALY_SHARDING_STRATEGY=RENDEZVOUS",
		"shard-count changes can still move assignments",
	} {
		if !strings.Contains(contextMessage, expected) {
			t.Errorf("context prompt does not contain %q", expected)
		}
	}
}

func TestRecommendationPromptRetainsDecisionFramework(t *testing.T) {
	for _, rule := range []string{
		"point anomalies are isolated deviations",
		"contextual anomalies depend on time",
		"collective anomalies are unusual sequences",
		"not synonymous with temporal collective anomalies",
		"treat them as a hypothesis to test",
		"Check missing values, gaps, sparsity, intermittency",
		"do not assume a model supports missing data",
		"Prefer univariate models for independent metrics",
		"Balance latency, memory/CPU, cardinality, interpretability",
		"Online models adapt during causal inference",
		"Offline models rely on refits",
		"Ask which misses and false alarms matter",
		"Validate against known incidents and normal periods",
		"Monitor alert quality and revisit assumptions",
		"fit_every longer than the selected inference date range",
	} {
		if !strings.Contains(contextMessage, rule) {
			t.Errorf("recommendation prompt lost decision rule %q", rule)
		}
	}
}
