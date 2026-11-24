package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func ptr[T any](v T) *T {
	return &v
}

// bindArguments decodes tool arguments into target through a generic JSON value, as
// mcp-go v0.43 did. mcp-go v1 unmarshals the raw arguments straight into target, so a
// whole number sent as a float (e.g. "limit": 100.0, which some clients do) no longer
// binds to an int field. Numbers pass through float64, so integers above 2^53 lose
// precision, also as before.
func bindArguments(req mcp.CallToolRequest, target any) error {
	generic := req.Params.Arguments
	if len(req.Params.RawArguments) > 0 {
		if err := json.Unmarshal(req.Params.RawArguments, &generic); err != nil {
			return err
		}
	}
	data, err := json.Marshal(generic)
	if err != nil {
		return fmt.Errorf("failed to marshal arguments: %w", err)
	}
	return json.Unmarshal(data, target)
}

// typedToolHandler replaces mcp.NewTypedToolHandler so that typed tools bind their
// arguments with bindArguments.
func typedToolHandler[T any](handler mcp.TypedToolHandlerFunc[T]) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args T
		if err := bindArguments(req, &args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to bind arguments: %v", err)), nil
		}
		return handler(ctx, req, args)
	}
}
