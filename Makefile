# hvu Makefile

BINARY_NAME := hvu
BUILD_DIR := bin
CMD_PATH := ./cmd/hvu

# Version info (from git or defaults)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go settings
GOFLAGS := -trimpath
LDFLAGS := -s -w \
	-X github.com/itsvictorfy/hvu/pkg/cli.Version=$(VERSION) \
	-X github.com/itsvictorfy/hvu/pkg/cli.Commit=$(COMMIT) \
	-X github.com/itsvictorfy/hvu/pkg/cli.BuildDate=$(BUILD_DATE)

# Tools versions
GOLANGCI_LINT_VERSION := v1.59.1

# Default target
.DEFAULT_GOAL := help

.PHONY: all
all: lint test build

## Build targets

.PHONY: build
build: ## Build binary for current platform
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)

.PHONY: build-linux
build-linux: ## Build for Linux (amd64 and arm64)
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_PATH)
	GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_PATH)

.PHONY: build-darwin
build-darwin: ## Build for macOS (amd64 and arm64)
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_PATH)
	GOOS=darwin GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_PATH)

.PHONY: build-windows
build-windows: ## Build for Windows (amd64)
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_PATH)

.PHONY: build-all
build-all: build-linux build-darwin build-windows ## Build for all platforms

.PHONY: install
install: build ## Install binary to GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	go install $(GOFLAGS) -ldflags "$(LDFLAGS)" $(CMD_PATH)

## Test targets

.PHONY: test
test: ## Run unit tests
	@echo "Running tests..."
	go test -race -cover ./...

.PHONY: test-short
test-short: ## Run unit tests (skip integration)
	@echo "Running short tests..."
	go test -short -race -cover ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: test-integration
test-integration: ## Run integration tests (requires network)
	@echo "Running integration tests..."
	go test -v -race -tags=integration ./test/...

## Lint targets

.PHONY: lint
lint: ## Run linters
	@echo "Running linters..."
	@if ! command -v golangci-lint &> /dev/null; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi
	golangci-lint run ./...

.PHONY: lint-fix
lint-fix: ## Run linters with auto-fix
	@echo "Running linters with fix..."
	golangci-lint run --fix ./...

## Development targets

.PHONY: fmt
fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

.PHONY: mod-tidy
mod-tidy: ## Tidy go modules
	@echo "Tidying go modules..."
	go mod tidy

.PHONY: mod-download
mod-download: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download

.PHONY: generate
generate: ## Run go generate
	@echo "Running go generate..."
	go generate ./...

## Clean targets

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	rm -rf upgrade-output/

.PHONY: clean-all
clean-all: clean ## Clean everything including cached test results
	go clean -testcache

## Help

.PHONY: help
help: ## Show this help
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
