package tools

import (
	"context"
	"encoding/json"
	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/resources"
	"github.com/mark3labs/mcp-go/mcp"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExcerptFindsRelevantLateParagraphAndSupportsContinuation(t *testing.T) {
	text := strings.Repeat("Introduction about something else.\n\n", 1000) + "CPU RAM dependency policies: 15% spikes and drops. " + strings.Repeat("é界", 500)
	got := excerpt("doc://test", text, "CPU RAM dependency policies", 250)
	if !strings.Contains(got.Text, "CPU RAM dependency policies") || got.Offset == 0 {
		t.Fatal("lost relevant late paragraph")
	}
	if !utf8.ValidString(got.Text) || len([]rune(got.Text)) > 250 || !got.Truncated {
		t.Fatal("invalid excerpt bounds")
	}
	rest := section(got.URI, text, got.NextOffset, 8000)
	if got.Text+rest.Text != string([]rune(text)[got.Offset:]) {
		t.Fatal("continuation lost text")
	}
}

func TestSearchCompactionPreservesTopKAndOrder(t *testing.T) {
	for _, query := range []string{"model", "temporal envelope", "data_range", "autotune"} {
		for _, k := range []int{3, 5, 30} {
			ranked, err := resources.SearchDocResources(query, k)
			if err != nil {
				t.Fatal(err)
			}
			result, err := handleSearchDocs()(context.Background(), mcp.CallToolRequest{}, SearchDocsArgs{Query: query, Limit: float64(k)})
			if err != nil {
				t.Fatal(err)
			}
			var excerpts []DocExcerpt
			if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &excerpts); err != nil {
				t.Fatal(err)
			}
			if len(excerpts) != len(ranked) {
				t.Fatalf("top-k changed: %d -> %d", len(ranked), len(excerpts))
			}
			total, original := 0, 0
			old := &mcp.CallToolResult{Content: []mcp.Content{}}
			for i, item := range excerpts {
				if item.URI != ranked[i].URI {
					t.Fatal("ranking changed")
				}
				raw, _ := resources.GetDocResourceContent(item.URI)
				full := raw.(mcp.TextResourceContents).Text
				old.Content = append(old.Content, mcp.EmbeddedResource{Type: "resource", Resource: raw})
				original += len(full)
				total += len([]rune(item.Text))
				if item.Text != string([]rune(full)[item.Offset:item.NextOffset]) {
					t.Fatal("excerpt is not verbatim source")
				}
			}
			if total > 24000 {
				t.Fatalf("text budget exceeded: %d", total)
			}
			oldJSON, _ := json.Marshal(old)
			newJSON, _ := json.Marshal(result)
			bytes := len(newJSON)
			t.Logf("query=%q top_k=%d actual=%d source_bytes=%d before_bytes=%d after_bytes=%d savings=%.1f%%", query, k, len(excerpts), original, len(oldJSON), bytes, 100*(1-float64(bytes)/float64(len(oldJSON))))
		}
	}

}

func TestDocSectionRejectsUnknownURIAndBounds(t *testing.T) {
	result, _ := handleReadDocSection(context.Background(), mcp.CallToolRequest{}, ReadDocArgs{URI: "file:///secret"})
	if !result.IsError {
		t.Fatal("unknown URI accepted")
	}
	result, _ = handleReadDocSection(context.Background(), mcp.CallToolRequest{}, ReadDocArgs{Offset: -1})
	if !result.IsError {
		t.Fatal("negative offset accepted")
	}
}
