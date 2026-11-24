package vmanomaly

import (
	"context"
	"net/http"
)

type requestHeadersKey struct{}

// WithRequestHeaders returns a context whose vmanomaly API calls carry headers.
// They override the client's static headers and bearer token on collision.
// The MCP server uses it to forward MCP_PASSTHROUGH_HEADERS from each request.
func WithRequestHeaders(ctx context.Context, headers http.Header) context.Context {
	if len(headers) == 0 {
		return ctx
	}
	return context.WithValue(ctx, requestHeadersKey{}, headers.Clone())
}

func requestHeadersFromContext(ctx context.Context) http.Header {
	headers, _ := ctx.Value(requestHeadersKey{}).(http.Header)
	return headers
}
