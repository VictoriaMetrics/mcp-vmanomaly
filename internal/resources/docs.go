package resources

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Embed only files the resource loader can serve. Documentation images remain
// in the repository but do not need to increase every binary and image by
// several megabytes.
//
//go:embed docs/*.md docs/*/*.md docs/*/*/*.md docs/*/*/*/*.md
var DocsDir embed.FS

const (
	docsURIPrefix              = "docs://"
	maxMarkdownDescriptionSize = 4096
)

var (
	docStoreOnce sync.Once
	docStoreErr  error
	resources    map[string]mcp.Resource
	contents     map[string]mcp.ResourceContents
	documents    map[string]DocFileInfo

	searchIndexOnce sync.Once
	searchIndexErr  error
	searchIndex     bleve.Index
)

func ensureDocStore() error {
	docStoreOnce.Do(func() {
		docFiles, err := ListDocFiles()
		if err != nil {
			docStoreErr = fmt.Errorf("error listing docs files: %w", err)
			return
		}

		resources = make(map[string]mcp.Resource, len(docFiles))
		contents = make(map[string]mcp.ResourceContents, len(docFiles))
		documents = make(map[string]DocFileInfo, len(docFiles))
		for _, docFile := range docFiles {
			resourceURI := fmt.Sprintf("%s%s#%d", docsURIPrefix, docFile.Path, docFile.ChunkNum)
			resources[resourceURI] = mcp.NewResource(
				resourceURI,
				docFile.Name,
				mcp.WithMIMEType("text/markdown"),
				mcp.WithResourceDescription(docFile.Content[:min(len(docFile.Content), maxMarkdownDescriptionSize)]),
			)
			contents[resourceURI] = mcp.TextResourceContents{
				URI:      resourceURI,
				MIMEType: "text/markdown",
				Text:     docFile.Content,
			}
			documents[resourceURI] = docFile
		}
	})
	return docStoreErr
}

func ensureSearchIndex() error {
	if err := ensureDocStore(); err != nil {
		return err
	}

	searchIndexOnce.Do(func() {
		index, err := bleve.NewMemOnly(bleve.NewIndexMapping())
		if err != nil {
			searchIndexErr = fmt.Errorf("error creating index: %w", err)
			return
		}

		for resourceURI, docFile := range documents {
			if err := index.Index(resourceURI, docFile); err != nil {
				_ = index.Close()
				searchIndexErr = fmt.Errorf("error indexing file %s: %w", docFile.Path, err)
				return
			}
		}
		searchIndex = index
	})
	return searchIndexErr
}

// RegisterDocsResources registers embedded documentation resources without
// eagerly constructing the full-text search index. Search indexing is deferred
// until vmanomaly_search_docs is called for the first time.
func RegisterDocsResources(s *server.MCPServer) error {
	if err := ensureDocStore(); err != nil {
		return err
	}

	resourceURIs := make([]string, 0, len(resources))
	for resourceURI := range resources {
		resourceURIs = append(resourceURIs, resourceURI)
	}
	sort.Strings(resourceURIs)
	for _, resourceURI := range resourceURIs {
		s.AddResource(resources[resourceURI], docResourcesHandler)
	}
	slog.Info("Registered documentation resources", "count", len(resourceURIs))
	return nil
}

// SearchDocResources searches documentation using full-text search
func SearchDocResources(query string, limit int) ([]mcp.Resource, error) {
	if err := ensureSearchIndex(); err != nil {
		return nil, err
	}
	searchQuery := bleve.NewMatchQuery(query)
	searchQuery.Fuzziness = 1
	searchRequest := bleve.NewSearchRequest(searchQuery)
	searchRequest.Size = limit
	searchResults, err := searchIndex.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("error searching index: %w", err)
	}
	if searchResults.Total == 0 {
		return nil, fmt.Errorf("no results found for query: %s", query)
	}
	results := make([]mcp.Resource, 0)
	for _, hit := range searchResults.Hits {
		if len(results) >= limit {
			break
		}
		resource, ok := resources[hit.ID]
		if !ok {
			continue
		}
		results = append(results, resource)
	}
	return results, nil
}

// docResourcesHandler handles ReadResource requests for documentation
func docResourcesHandler(_ context.Context, rrr mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	content, err := GetDocResourceContent(rrr.Params.URI)
	if err != nil {
		return nil, fmt.Errorf("error getting doc resource content: %w", err)
	}
	return []mcp.ResourceContents{content}, nil
}

// GetDocResourceContent retrieves cached resource content by URI
func GetDocResourceContent(uri string) (mcp.ResourceContents, error) {
	if err := ensureDocStore(); err != nil {
		return nil, err
	}
	content, ok := contents[uri]
	if !ok {
		return nil, fmt.Errorf("resource not found: %s", uri)
	}
	return content, nil
}

// GetDocFileContent reads a file from the embedded filesystem
func GetDocFileContent(path string) (string, error) {
	file, err := fs.ReadFile(DocsDir, path)
	if err != nil {
		return "", fmt.Errorf("error reading file %s: %w", path, err)
	}
	return string(file), nil
}

// DocFileInfo represents a documentation chunk
type DocFileInfo struct {
	Path     string `json:"path"`
	ChunkNum int    `json:"chunk_num"`
	Content  string `json:"content"`
	Name     string `json:"name"`
}

// ListDocFiles scans the embedded filesystem and chunks all markdown files
func ListDocFiles() ([]DocFileInfo, error) {
	docs := make([]DocFileInfo, 0)

	// Walk the docs directory
	err := fs.WalkDir(DocsDir, "docs", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Only process markdown files
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".md") {
			return nil
		}

		content, err := GetDocFileContent(path)
		if err != nil {
			return fmt.Errorf("error reading file %s: %w", path, err)
		}

		chunks, err := splitMarkdown(content)
		if err != nil {
			return fmt.Errorf("error splitting file %s: %w", path, err)
		}

		for chunkNum, chunkContent := range chunks {
			name := ""
			for line := range strings.Lines(chunkContent) {
				if strings.TrimSpace(line) == "" {
					continue
				}
				if !strings.HasPrefix(line, "#") {
					break
				}
				title := strings.TrimSpace(strings.Trim(line, "# "))
				name = fmt.Sprintf("%s / %s", name, title)
			}
			name = strings.Trim(name, "/ ")
			if name == "" {
				name = filepath.Base(path)
			}

			docs = append(docs, DocFileInfo{
				Path:     path,
				ChunkNum: chunkNum,
				Content:  chunkContent,
				Name:     name,
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking docs directory: %w", err)
	}

	return docs, nil
}
