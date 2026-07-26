package resources

import (
	"context"
	"io/fs"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestEmbeddedDocumentationContainsMarkdownOnly(t *testing.T) {
	err := filepath.WalkDir("docs", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		if _, err := DocsDir.ReadFile(filepath.ToSlash(path)); err != nil {
			t.Errorf("Markdown file %q is missing from embedded documentation: %v", path, err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to inspect documentation: %v", err)
	}

	const imagePath = "docs/anomaly-detection/vmanomaly-ui-overview.webp"
	if _, err := DocsDir.ReadFile(imagePath); err == nil {
		t.Fatalf("documentation image %q must not be embedded", imagePath)
	}
}

func resetDocumentationState(t *testing.T) {
	t.Helper()
	if searchIndex != nil {
		if err := searchIndex.Close(); err != nil {
			t.Fatalf("failed to close search index: %v", err)
		}
	}
	docStoreOnce = sync.Once{}
	docStoreErr = nil
	resources = nil
	contents = nil
	documents = nil
	searchIndexOnce = sync.Once{}
	searchIndexErr = nil
	searchIndex = nil
}

func TestDocumentationSearchIndexIsLazy(t *testing.T) {
	resetDocumentationState(t)
	mcpServer := server.NewMCPServer("test", "test")
	if err := RegisterDocsResources(mcpServer); err != nil {
		t.Fatalf("RegisterDocsResources failed: %v", err)
	}
	if searchIndex != nil {
		t.Fatal("resource registration must not eagerly construct the search index")
	}

	results, err := SearchDocResources("prophet seasonality", 2)
	if err != nil {
		t.Fatalf("SearchDocResources failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected documentation search results")
	}
	if searchIndex == nil {
		t.Fatal("documentation search must initialize the index")
	}
}

func firstDocResourceURI(t *testing.T) string {
	t.Helper()
	if err := ensureDocStore(); err != nil {
		t.Fatalf("failed to initialize documentation store: %v", err)
	}
	for uri := range contents {
		return uri
	}
	t.Fatal("documentation store is empty")
	return ""
}

// TestGetDocResourceContent tests the GetDocResourceContent function
func TestGetDocResourceContent(t *testing.T) {
	testURI := firstDocResourceURI(t)

	// Test cases
	testCases := []struct {
		name        string
		uri         string
		expectError bool
	}{
		{
			name:        "Valid URI",
			uri:         testURI,
			expectError: false,
		},
		{
			name:        "Invalid URI",
			uri:         "docs://nonexistent.md#0",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the function
			content, err := GetDocResourceContent(tc.uri)

			// Check for errors
			if tc.expectError {
				if err == nil {
					t.Error("Expected an error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check the content
			if content == nil {
				t.Fatal("Expected non-nil content")
			}

			// Check that the content is a TextResourceContents
			textContent, ok := content.(mcp.TextResourceContents)
			if !ok {
				t.Fatal("Expected TextResourceContents, got different content type")
			}

			// Check the URI
			if textContent.URI != tc.uri {
				t.Errorf("Expected URI '%s', got: '%s'", tc.uri, textContent.URI)
			}
		})
	}
}

// TestDocResourcesHandler tests the docResourcesHandler function
func TestDocResourcesHandler(t *testing.T) {
	testURI := firstDocResourceURI(t)

	// Test cases
	testCases := []struct {
		name        string
		uri         string
		expectError bool
	}{
		{
			name:        "Valid URI",
			uri:         testURI,
			expectError: false,
		},
		{
			name:        "Invalid URI",
			uri:         "docs://nonexistent.md#0",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock request
			req := mcp.ReadResourceRequest{}
			req.Params.URI = tc.uri

			// Call the handler
			result, err := docResourcesHandler(context.Background(), req)

			// Check for errors
			if tc.expectError {
				if err == nil {
					t.Error("Expected an error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check the result
			if len(result) != 1 {
				t.Fatalf("Expected 1 result, got: %d", len(result))
			}

			// Check that the content is correct
			content := result[0]
			textContent, ok := content.(mcp.TextResourceContents)
			if !ok {
				t.Fatal("Expected TextResourceContents, got different content type")
			}

			// Check the URI
			if textContent.URI != tc.uri {
				t.Errorf("Expected URI '%s', got: '%s'", tc.uri, textContent.URI)
			}
		})
	}
}

// TestGetDocFileContent tests reading from the embedded filesystem
func TestGetDocFileContent(t *testing.T) {
	// This test verifies that we can read files from the embedded docs
	// Since we can't predict exact file paths, we'll just test that
	// the function works with non-existent files

	t.Run("Nonexistent file returns error", func(t *testing.T) {
		_, err := GetDocFileContent("docs/nonexistent-file.md")
		if err == nil {
			t.Error("Expected error for nonexistent file, got nil")
		}
	})
}
