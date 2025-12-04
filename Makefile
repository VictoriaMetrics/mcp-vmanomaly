.PHONY: build build-all run test test-coverage test-integration test-integration-docker test-all ci clean install setup-ci fmt vet lint check update-docs help

# Binary name
BINARY_NAME=mcp-vmanomaly
BUILD_DIR=bin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet
GOMOD=$(GOCMD) mod

# Local bin for CI tools
LOCALBIN = $(shell pwd)/.bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# CI Tool versions
GOLANGCI_LINT_VERSION ?= v1.62.2
WWHRD_VERSION ?= v0.4.0
GOVULNCHECK_VERSION ?= latest

# CI Tool paths
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
WWHRD = $(LOCALBIN)/wwhrd
GOVULNCHECK = $(LOCALBIN)/govulncheck

# Main package path
MAIN_PATH=./cmd/$(BINARY_NAME)

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

build-all: ## Build binaries for all platforms (Linux, macOS, Windows)
	@echo "Building $(BINARY_NAME) for all platforms..."
	@mkdir -p $(BUILD_DIR)
	@echo "Building for Linux AMD64..."
	@GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	@echo "Building for Linux ARM64..."
	@GOOS=linux GOARCH=arm64 $(GOBUILD) -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PATH)
	@echo "Building for macOS AMD64..."
	@GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	@echo "Building for macOS ARM64 (Apple Silicon)..."
	@GOOS=darwin GOARCH=arm64 $(GOBUILD) -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	@echo "Building for Windows AMD64..."
	@GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	@echo "Build complete! Binaries in $(BUILD_DIR)/"
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME)-*

run: ## Run the application
	@echo "Running $(BINARY_NAME)..."
	$(GORUN) $(MAIN_PATH)

test: ## Run tests
	@bash ./scripts/test-all.sh

test-coverage: test ## Run tests with coverage report
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-integration: ## Run integration tests (requires vmanomaly server running)
	@echo "Running integration tests..."
	@echo "Note: Requires VMANOMALY_ENDPOINT to be set (default: http://localhost:8490)"
	$(GOTEST) -v -tags=integration -race ./...

test-integration-docker: ## Run integration tests with Docker
	@echo "Starting test environment..."
	@docker-compose -f testdata/docker-compose.test.yml up -d
	@echo "Waiting for services to be healthy..."
	@sleep 15
	@echo "Running integration tests..."
	@VMANOMALY_ENDPOINT=http://localhost:8490 $(GOTEST) -v -tags=integration ./... || \
		(echo "Tests failed, cleaning up..." && docker-compose -f testdata/docker-compose.test.yml down && exit 1)
	@echo "Stopping test environment..."
	@docker-compose -f testdata/docker-compose.test.yml down
	@echo "Integration tests complete"

test-all: test test-integration ## Run all tests (unit + integration)

ci: ## CI pipeline (lint + check + tests + integration with Docker)
	@echo "=== Running CI Pipeline ==="
	@echo "Step 1: Formatting and linting..."
	@$(MAKE) lint
	@echo "Step 2: Security checks..."
	@$(MAKE) check
	@echo "Step 3: Unit tests..."
	@$(MAKE) test

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR) $(LOCALBIN)
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

install: ## Install dependencies
	@echo "Installing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "Dependencies installed"

setup-ci: $(GOLANGCI_LINT) $(WWHRD) $(GOVULNCHECK) ## Install CI tools to .bin/

$(GOLANGCI_LINT): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(WWHRD): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/frapposelli/wwhrd@$(WWHRD_VERSION)

$(GOVULNCHECK): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

fmt: ## Format code
	@echo "Formatting code..."
	$(GOFMT) ./...

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...

lint: fmt vet $(GOLANGCI_LINT) ## Run all linters
	$(GOLANGCI_LINT) run --config .golangci.yml

check: $(WWHRD) $(GOVULNCHECK) ## Check licenses and vulnerabilities
	$(WWHRD) check -f .wwhrd.yml
	$(GOVULNCHECK) ./...

update-docs: ## Update embedded vmanomaly documentation
	@bash ./scripts/update-docs.sh

dev: ## Run in development mode with auto-reload (requires air)
	@which air > /dev/null || (echo "air not installed. Run: go install github.com/cosmtrek/air@latest" && exit 1)
	@air

# Docker targets
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):latest .

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	docker run --rm -e VMANOMALY_ENDPOINT=$(VMANOMALY_ENDPOINT) $(BINARY_NAME):latest

.DEFAULT_GOAL := help
