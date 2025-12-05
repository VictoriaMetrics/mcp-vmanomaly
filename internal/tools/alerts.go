package tools

import (
	"context"
	"fmt"

	"github.com/VictoriaMetrics-Community/mcp-vmanomaly/internal/vmanomaly"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type GenerateAlertRuleArgs struct {
	Step             string  `json:"step" jsonschema:"required,description=Query step/resolution (e.g. '1s' '1m' '5m')"`
	Query            string  `json:"query" jsonschema:"required,description=PromQL query to include in alert description"`
	AnomalyThreshold float64 `json:"anomaly_threshold,omitempty" jsonschema:"description=Anomaly score threshold (default: 1.0)"`
	RuleName         string  `json:"rule_name,omitempty" jsonschema:"description=Custom alert rule name"`
	GroupName        string  `json:"group_name,omitempty" jsonschema:"description=VMAlert rule group name (default: 'VMAnomalyAlerts')"`
	RuleDescription  string  `json:"rule_description,omitempty" jsonschema:"description=Custom alert description/summary"`
	InferEvery       string  `json:"infer_every,omitempty" jsonschema:"description=Inference cadence (defaults to step value)"`
}

func RegisterAlertTools(s *server.MCPServer, client *vmanomaly.Client) {
	generateAlertRuleTool := mcp.NewTool(
		"vmanomaly_generate_alert_rule",
		mcp.WithDescription("Generate a VMAlert rule YAML configuration for anomaly score alerting. Creates a production-ready vmalert rule that triggers when anomaly_score exceeds the threshold. Use this to set up alerting for anomalies detected by vmanomaly."),
		mcp.WithInputSchema[GenerateAlertRuleArgs](),
	)
	s.AddTool(generateAlertRuleTool, mcp.NewTypedToolHandler(handleGenerateAlertRule(client)))
}

func handleGenerateAlertRule(client *vmanomaly.Client) func(ctx context.Context, req mcp.CallToolRequest, args GenerateAlertRuleArgs) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest, args GenerateAlertRuleArgs) (*mcp.CallToolResult, error) {
		alertReq := &vmanomaly.AlertRuleRequest{
			Step:  args.Step,
			Query: args.Query,
		}

		if args.AnomalyThreshold > 0 {
			alertReq.AnomalyThreshold = &args.AnomalyThreshold
		}
		if args.RuleName != "" {
			alertReq.RuleName = &args.RuleName
		}
		if args.GroupName != "" {
			alertReq.GroupName = &args.GroupName
		}
		if args.RuleDescription != "" {
			alertReq.RuleDescription = &args.RuleDescription
		}
		if args.InferEvery != "" {
			alertReq.InferEvery = &args.InferEvery
		}

		yamlConfig, err := client.GenerateAlertRule(ctx, alertReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to generate alert rule: %v", err)), nil
		}

		resultMsg := fmt.Sprintf("Generated VMAlert Rule:\n\n```yaml\n%s\n```\n\nSave this to a .yaml file and configure vmalert to load it.", yamlConfig)
		return mcp.NewToolResultText(resultMsg), nil
	}
}
