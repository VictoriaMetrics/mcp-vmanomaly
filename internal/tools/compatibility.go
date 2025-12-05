package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/VictoriaMetrics-Community/mcp-vmanomaly/internal/vmanomaly"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type CheckCompatibilityArgs struct {
	VersionTo string `json:"version_to,omitempty" jsonschema_description:"Optional target version to check compatibility against. If omitted, checks against current runtime version."`
}

type CheckCompatibilityResponse struct {
	// Human-readable summary for LLM understanding
	Summary string `json:"summary" jsonschema_description:"Human-readable summary of compatibility status and required actions"`

	Status          string   `json:"status" jsonschema_description:"Compatibility status: 'compatible', 'incompatible', or 'no_state'"`
	RuntimeVersion  string   `json:"runtime_version" jsonschema_description:"Runtime version being checked against"`
	StoredVersion   *string  `json:"stored_version,omitempty" jsonschema_description:"Version of stored state (if any)"`
	HasState        bool     `json:"has_state" jsonschema_description:"Whether persisted state exists"`
	IsCompatible    bool     `json:"is_compatible" jsonschema_description:"Whether state is compatible with runtime"`
	DropEverything  bool     `json:"drop_everything" jsonschema_description:"Whether ALL state must be dropped"`
	ModelsToPurge   []string `json:"models_to_purge,omitempty" jsonschema_description:"Model aliases that need purging"`
	PurgeReaderData bool     `json:"purge_reader_data" jsonschema_description:"Whether reader data needs purging"`
	Reason          *string  `json:"reason,omitempty" jsonschema_description:"Explanation of incompatibility"`
}

func RegisterCompatibilityTools(s *server.MCPServer, client *vmanomaly.Client) {
	checkCompatibilityTool := mcp.NewTool(
		"vmanomaly_check_compatibility",
		mcp.WithDescription("Check if persisted vmanomaly state is compatible with the current or target runtime version. Returns compatibility status and required migration actions."),
		mcp.WithInputSchema[CheckCompatibilityArgs](),
		mcp.WithOutputSchema[CheckCompatibilityResponse](),
	)
	s.AddTool(checkCompatibilityTool, mcp.NewStructuredToolHandler(handleCheckCompatibility(client)))
}

func handleCheckCompatibility(client *vmanomaly.Client) mcp.StructuredToolHandlerFunc[CheckCompatibilityArgs, CheckCompatibilityResponse] {
	return func(ctx context.Context, req mcp.CallToolRequest, args CheckCompatibilityArgs) (CheckCompatibilityResponse, error) {
		var versionTo *string
		if args.VersionTo != "" {
			versionTo = &args.VersionTo
		}

		result, err := client.Compatibility(ctx, versionTo)
		if err != nil {
			return CheckCompatibilityResponse{}, fmt.Errorf("compatibility check failed: %w", err)
		}

		resp := CheckCompatibilityResponse{
			RuntimeVersion:  result.RuntimeVersion,
			StoredVersion:   result.StoredVersion,
			HasState:        result.GlobalCheck.HasState,
			IsCompatible:    result.GlobalCheck.IsCompatible,
			DropEverything:  result.GlobalCheck.DropEverything,
			Reason:          result.GlobalCheck.Reason,
			PurgeReaderData: false,
			ModelsToPurge:   []string{},
		}

		if result.ComponentAssessment != nil {
			resp.ModelsToPurge = result.ComponentAssessment.ModelsToPurge
			resp.PurgeReaderData = result.ComponentAssessment.ShouldPurgeReaderData
		}

		if !resp.HasState {
			resp.Status = "no_state"
		} else if resp.IsCompatible {
			resp.Status = "compatible"
		} else {
			resp.Status = "incompatible"
		}

		resp.Summary = buildCompatibilitySummary(resp)

		return resp, nil
	}
}

func buildCompatibilitySummary(r CheckCompatibilityResponse) string {
	var sb strings.Builder

	switch r.Status {
	case "no_state":
		sb.WriteString("No persisted state found (fresh install). ")
		sb.WriteString("System is ready to use with any configuration.")
	case "compatible":
		sb.WriteString(fmt.Sprintf("State is COMPATIBLE with runtime %s. ", r.RuntimeVersion))
		sb.WriteString("No migration actions required.")
	case "incompatible":
		sb.WriteString(fmt.Sprintf("State is INCOMPATIBLE with runtime %s. ", r.RuntimeVersion))

		if r.DropEverything {
			sb.WriteString("CRITICAL: All persisted state must be dropped before upgrade. ")
		} else {
			var actions []string
			if len(r.ModelsToPurge) > 0 {
				actions = append(actions, fmt.Sprintf("purge models: %s", strings.Join(r.ModelsToPurge, ", ")))
			}
			if r.PurgeReaderData {
				actions = append(actions, "purge reader data")
			}
			if len(actions) > 0 {
				sb.WriteString(fmt.Sprintf("Required actions: %s. ", strings.Join(actions, "; ")))
			}
		}

		if r.Reason != nil {
			sb.WriteString(fmt.Sprintf("Reason: %s", *r.Reason))
		}
	}

	return sb.String()
}
