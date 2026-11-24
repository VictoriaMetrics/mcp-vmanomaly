// Package auth protects the MCP HTTP endpoints with a static bearer token.
package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"path"
	"strings"
)

// Middleware returns an HTTP middleware that requires the header
// "Authorization: Bearer <token>" on MCP endpoints. Health and metrics
// endpoints stay public so probes and scrapers work without the token.
func Middleware(token string) func(http.Handler) http.Handler {
	// Comparing fixed-size digests keeps the check constant-time regardless of token length.
	expected := sha256.Sum256([]byte(token))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				reject(w, r, "missing Authorization header")
				return
			}
			scheme, credentials, ok := strings.Cut(authHeader, " ")
			if !ok || !strings.EqualFold(scheme, "Bearer") {
				reject(w, r, "invalid Authorization header format")
				return
			}
			presented := sha256.Sum256([]byte(credentials))
			if subtle.ConstantTimeCompare(presented[:], expected[:]) != 1 {
				reject(w, r, "invalid token")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isPublicPath reports whether a request path skips authentication.
// The path is cleaned first, so "/health/../mcp" is not treated as public.
func isPublicPath(requestPath string) bool {
	p := path.Clean(requestPath)
	return p == "/metrics" || p == "/health" || strings.HasPrefix(p, "/health/")
}

func reject(w http.ResponseWriter, r *http.Request, reason string) {
	slog.Warn("Authentication failed",
		"reason", reason,
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
	)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Bearer realm="mcp-vmanomaly"`)
	w.WriteHeader(http.StatusUnauthorized)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": reason}); err != nil {
		slog.Error("Failed to write unauthorized response", "error", err)
	}
}
