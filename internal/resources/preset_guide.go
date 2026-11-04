package resources

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const PresetGuideURI = "vmanomaly://guides/preset/v1"

// This routing hint is short enough for MCP initialization; the workflow is retrieved on demand.
const PresetGuideInstructions = "For requests to discover signals and build an anomaly-detection preset, call vmanomaly_get_preset_guide when available before discovery or configuration suggestions, then follow it through artifact generation, using declared draft placeholders for unknown writer settings unless deployment-ready output is explicitly requested. Workspace edits are not preset completion."

//go:embed guides/preset-v1.md
var presetGuideText string

//go:embed guides/preset-v1.json
var presetGuideMetadata json.RawMessage

// PresetGuide returns a fresh envelope; tool calls cannot mutate the embedded snapshot.
func PresetGuide() map[string]any {
	return map[string]any{
		"uri":      PresetGuideURI,
		"metadata": json.RawMessage(append([]byte(nil), presetGuideMetadata...)),
		"guide":    presetGuideText,
	}
}

func RegisterPresetGuideResource(s *server.MCPServer) {
	s.AddResource(mcp.NewResource(PresetGuideURI, "Preset workflow v1",
		mcp.WithMIMEType("text/markdown"),
		mcp.WithResourceDescription("Versioned preset workflow from VictoriaMetrics/skills; also available via vmanomaly_get_preset_guide.")),
		presetGuideResourceHandler)
}

func presetGuideResourceHandler(_ context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	if request.Params.URI != PresetGuideURI {
		return nil, fmt.Errorf("unsupported preset guide URI: %s", request.Params.URI)
	}
	return []mcp.ResourceContents{mcp.TextResourceContents{
		URI: PresetGuideURI, MIMEType: "text/markdown", Text: presetGuideText,
	}}, nil
}
