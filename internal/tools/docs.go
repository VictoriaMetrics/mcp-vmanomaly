package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"unicode"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/resources"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ============================================================================
// Documentation Search Tool Arguments (Struct-based schemas)
// ============================================================================

// SearchDocsArgs defines arguments for search_docs tool
type SearchDocsArgs struct {
	Query string  `json:"query" jsonschema_description:"Search query for vmanomaly documentation. Supports keywords phrases or natural language questions. Example queries: 'temporal envelope parameters' 'how to configure seasonality' 'online models' 'installation requirements' 'troubleshooting errors'. Uses fuzzy matching to find relevant documentation chunks."`
	Limit float64 `json:"limit,omitempty" jsonschema_description:"Maximum number of documentation resources to return. Range: 1-100. Default: 5. Ranking and requested top-k are preserved; excerpts share a bounded text budget."`
}

// ============================================================================
// Tool Registration Functions
// ============================================================================

// RegisterDocsTool registers the documentation search tool
func RegisterDocsTool(s *server.MCPServer) {
	searchDocsTool := mcp.NewTool(
		"vmanomaly_search_docs",
		mcp.WithDescription("Search vmanomaly documentation using full-text search with fuzzy matching. Returns ranked excerpts with source URIs and character offsets. Use vmanomaly_read_doc_section for additional text; excerpts are not complete documentation. Use this when you need information about model parameters, configuration syntax, troubleshooting, or feature explanations."),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           "Search vmanomaly Docs",
			ReadOnlyHint:    ptr(true),
			DestructiveHint: ptr(false),
			OpenWorldHint:   ptr(false),
		}),
		mcp.WithInputSchema[SearchDocsArgs](),
	)
	s.AddTool(searchDocsTool, mcp.NewTypedToolHandler(handleSearchDocs()))
	s.AddTool(mcp.NewTool("vmanomaly_read_doc_section",
		mcp.WithDescription("Read a bounded documentation section by source URI and character offset returned by search. Follow next_offset for continuation; fetch relevant sections before relying on truncated guidance."),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: ptr(true), DestructiveHint: ptr(false), OpenWorldHint: ptr(false)}),
		mcp.WithInputSchema[ReadDocArgs]()), mcp.NewTypedToolHandler(handleReadDocSection))
}

// ============================================================================
// Tool Handlers
// ============================================================================

// handleSearchDocs handles the search_docs tool
func handleSearchDocs() func(ctx context.Context, req mcp.CallToolRequest, args SearchDocsArgs) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest, args SearchDocsArgs) (*mcp.CallToolResult, error) {
		// Validate and set defaults
		limit := int(args.Limit)
		if limit < 1 {
			limit = 5 // default
		} else if limit > 100 {
			limit = 100
		}

		// Search documentation
		rs, err := resources.SearchDocResources(args.Query, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Search failed: %v", err)), nil
		}

		excerpts := make([]DocExcerpt, 0, len(rs))
		for resultIndex, resource := range rs {
			content, err := resources.GetDocResourceContent(resource.URI)
			if err != nil {
				slog.Warn("Failed to load documentation search result", "result_index", resultIndex)
				continue
			}
			doc, ok := content.(mcp.TextResourceContents)
			if !ok {
				continue
			}
			budget := 24000 / max(1, len(rs))
			excerpts = append(excerpts, excerpt(resource.URI, doc.Text, args.Query, budget))
		}
		if len(excerpts) == 0 {
			return mcp.NewToolResultText("No documentation found."), nil
		}
		data, err := json.Marshal(excerpts)
		if err != nil {
			return nil, err
		}
		result := mcp.NewToolResultText(string(data))

		return result, nil
	}
}

// Offsets and budgets are Unicode code points, not bytes. Search never changes ranking.
type DocExcerpt struct {
	URI        string `json:"uri"`
	Text       string `json:"text"`
	Offset     int    `json:"offset"`
	NextOffset int    `json:"next_offset"`
	TotalChars int    `json:"total_chars"`
	Truncated  bool   `json:"truncated"`
}

type ReadDocArgs struct {
	URI    string `json:"uri" jsonschema:"required,description=Exact documentation URI returned by search"`
	Offset int    `json:"offset,omitempty" jsonschema:"description=Zero-based character offset (default 0)"`
	Limit  int    `json:"limit,omitempty" jsonschema:"description=Character count, default 4000, maximum 8000"`
}

func section(uri, text string, offset, limit int) DocExcerpt {
	runes := []rune(text)
	start := min(max(0, offset), len(runes))
	end := min(start+limit, len(runes))
	return DocExcerpt{URI: uri, Text: string(runes[start:end]), Offset: start, NextOffset: end,
		TotalChars: len(runes), Truncated: start > 0 || end < len(runes)}
}

func excerpt(uri, text, query string, limit int) DocExcerpt {
	if len([]rune(text)) <= limit {
		return section(uri, text, 0, limit)
	}
	// Prefer the paragraph matching the most query terms; avoid bias toward document introductions.
	terms := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	bestOffset, bestScore, offset := 0, 0, 0
	for _, paragraph := range strings.Split(text, "\n\n") {
		lower := strings.ToLower(paragraph)
		score := 0
		for _, term := range terms {
			if len([]rune(term)) > 2 && strings.Contains(lower, term) {
				score++
			}
		}
		if score > bestScore {
			bestScore, bestOffset = score, offset
		}
		offset += len([]rune(paragraph)) + 2
	}
	return section(uri, text, bestOffset, limit)
}

func handleReadDocSection(ctx context.Context, req mcp.CallToolRequest, args ReadDocArgs) (*mcp.CallToolResult, error) {
	if args.Offset < 0 {
		return mcp.NewToolResultError("offset must be non-negative"), nil
	}
	content, err := resources.GetDocResourceContent(args.URI)
	if err != nil {
		return mcp.NewToolResultError("Unknown documentation URI; use search first."), nil
	}
	doc, ok := content.(mcp.TextResourceContents)
	if !ok {
		return mcp.NewToolResultError("Documentation is not text."), nil
	}
	limit := args.Limit
	if limit <= 0 {
		limit = 4000
	}
	data, err := json.Marshal(section(args.URI, doc.Text, args.Offset, min(limit, 8000)))
	if err != nil {
		return nil, err
	}
	return mcp.NewToolResultText(string(data)), nil
}
