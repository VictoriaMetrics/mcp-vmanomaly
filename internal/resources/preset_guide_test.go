package resources

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestPresetGuideSnapshotAndResource(t *testing.T) {
	var metadata struct {
		Version          int    `json:"version"`
		SHA256           string `json:"sha256"`
		SourceRepository string `json:"source_repository"`
		SourceRevision   string `json:"source_revision"`
	}
	if err := json.Unmarshal(presetGuideMetadata, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.Version != 1 || metadata.SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(presetGuideText))) {
		t.Fatal("guide version or checksum differs from the canonical snapshot")
	}
	if metadata.SourceRepository != "https://github.com/VictoriaMetrics/skills" || len(metadata.SourceRevision) != 40 {
		t.Fatal("missing canonical source provenance")
	}
	// Keep the on-demand guide compact enough for a single bounded context insertion.
	const maxGuideBytes = 12 * 1024
	if len(presetGuideText) == 0 || len(presetGuideText) > maxGuideBytes {
		t.Fatalf("unexpected guide size: %d", len(presetGuideText))
	}
	request := mcp.ReadResourceRequest{}
	request.Params.URI = PresetGuideURI
	result, err := presetGuideResourceHandler(context.Background(), request)
	if err != nil || len(result) != 1 {
		t.Fatalf("read resource: %v %v", result, err)
	}
	if result[0].(mcp.TextResourceContents).Text != PresetGuide()["guide"] {
		t.Fatal("resource and tool guide differ")
	}
	request.Params.URI += "/unsupported"
	if _, err := presetGuideResourceHandler(context.Background(), request); err == nil {
		t.Fatal("unknown guide URI accepted")
	}
	if !strings.Contains(PresetGuideInstructions, "vmanomaly_get_preset_guide") {
		t.Fatal("initialization does not route to the retrievable guide")
	}
}
