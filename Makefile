# nkk: Production-ready Makefile for Browser Automation Agent
# Enhanced with monitoring, quality checks, and deployment automation

.PHONY: help test unit integration e2e clean setup docker-up docker-down build run lint fmt vet security coverage benchmark monitor

# Application configuration
APP_NAME := agent
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GO_VERSION := $(shell go version | cut -d' ' -f3)

# Build configuration
BUILD_DIR := dist
DOCKER_IMAGE := browser-automation-agent
DOCKER_TAG := $(VERSION)

# Go build flags
LDFLAGS := -ldflags="-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"
BUILD_FLAGS := -trimpath $(LDFLAGS)

# Test configuration
TEST_TIMEOUT := 10m
COVERAGE_OUT := coverage.out
COVERAGE_HTML := coverage.html

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[1;33m
BLUE := \033[0;34m
NC := \033[0m # No Color

help: ## Show this help message
	@echo '${BLUE}Browser Automation Agent - Production Build System${NC}'
	@echo '${BLUE}=================================================${NC}'
	@echo ''
	@echo '${GREEN}Available targets:${NC}'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# =============================================================================
# Build & Development
# =============================================================================

build: ## Build the application for all platforms
	@echo '${BLUE}Building $(APP_NAME) for all platforms...${NC}'
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 ./cmd/agent
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-arm64 ./cmd/agent
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 ./cmd/agent
	@CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)-darwin-arm64 ./cmd/agent
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe ./cmd/agent
	@echo '${GREEN}✓ Build completed for all platforms${NC}'

build-local: ## Build for local platform only
	@echo '${BLUE}Building $(APP_NAME) for local platform...${NC}'
	@mkdir -p $(BUILD_DIR)
	@go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME) ./cmd/agent
	@echo '${GREEN}✓ Local build completed${NC}'

run: setup ## Run the application locally
	@echo '${BLUE}Starting $(APP_NAME) with monitoring...${NC}'
	@echo '${YELLOW}Metrics: http://localhost:9090/metrics${NC}'
	@echo '${YELLOW}Health:  http://localhost:9090/health${NC}'
	@go run ./cmd/agent start

run-bg: setup ## Run the application in background
	@echo '${BLUE}Starting $(APP_NAME) in background mode...${NC}'
	@go run ./cmd/agent start --background

# =============================================================================
# Code Quality & Security
# =============================================================================

lint: ## Run linters
	@echo '${BLUE}Running linters...${NC}'
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo '${YELLOW}Installing golangci-lint...${NC}'; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		golangci-lint run ./...; \
	fi
	@echo '${GREEN}✓ Linting completed${NC}'

fmt: ## Format code
	@echo '${BLUE}Formatting code...${NC}'
	@go fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	else \
		go install golang.org/x/tools/cmd/goimports@latest; \
		goimports -w .; \
	fi
	@echo '${GREEN}✓ Code formatted${NC}'

vet: ## Run go vet
	@echo '${BLUE}Running go vet...${NC}'
	@go vet ./...
	@echo '${GREEN}✓ Vet completed${NC}'

security: ## Run security scan
	@echo '${BLUE}Running security scan...${NC}'
	@if command -v gosec >/dev/null 2>&1; then \
		gosec -quiet ./...; \
	else \
		echo '${YELLOW}Installing gosec...${NC}'; \
		go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest; \
		gosec -quiet ./...; \
	fi
	@echo '${GREEN}✓ Security scan completed${NC}'

coverage: ## Generate test coverage report
	@echo '${BLUE}Generating coverage report...${NC}'
	@go test -race -coverprofile=$(COVERAGE_OUT) -covermode=atomic ./...
	@go tool cover -html=$(COVERAGE_OUT) -o $(COVERAGE_HTML)
	@go tool cover -func=$(COVERAGE_OUT) | tail -1
	@echo '${GREEN}✓ Coverage report: $(COVERAGE_HTML)${NC}'

benchmark: ## Run benchmark tests
	@echo '${BLUE}Running benchmarks...${NC}'
	@go test -bench=. -benchmem ./...
	@echo '${GREEN}✓ Benchmarks completed${NC}'

check: lint vet security unit ## Run all code quality checks

# =============================================================================
# Monitoring & Operations
# =============================================================================

monitor: ## Open monitoring dashboards
	@echo '${BLUE}Opening monitoring dashboards...${NC}'
	@echo 'Metrics: http://localhost:9090/metrics'
	@echo 'Health:  http://localhost:9090/health'
	@open http://localhost:9090/metrics 2>/dev/null || echo 'Visit: http://localhost:9090/metrics'

health: ## Check application health
	@echo '${BLUE}Checking application health...${NC}'
	@curl -s http://localhost:9090/health | jq . 2>/dev/null || \
	curl -s http://localhost:9090/health || \
	echo '${RED}Application not running or not accessible${NC}'

metrics: ## Show current metrics
	@echo '${BLUE}Current application metrics:${NC}'
	@curl -s http://localhost:9090/metrics | grep -E '^(browser_pool|test_execution|http_request)' | head -20 || \
	echo '${RED}Metrics not available${NC}'

# =============================================================================
# Version & Environment
# =============================================================================

version: ## Show version information
	@echo '${BLUE}Version Information:${NC}'
	@echo '  App Version: $(VERSION)'
	@echo '  Git Commit:  $(GIT_COMMIT)'
	@echo '  Build Time:  $(BUILD_TIME)'
	@echo '  Go Version:  $(GO_VERSION)'

env: ## Show environment information
	@echo '${BLUE}Environment Information:${NC}'
	@echo '  GOOS:        $(shell go env GOOS)'
	@echo '  GOARCH:      $(shell go env GOARCH)'
	@echo '  CGO_ENABLED: $(shell go env CGO_ENABLED)'
	@echo '  GOPATH:      $(shell go env GOPATH)'
	@echo '  GOROOT:      $(shell go env GOROOT)'

# =============================================================================
# Environment Setup & Testing
# =============================================================================

setup: ## Setup development environment
	@echo "${GREEN}Setting up development environment...${NC}"
	@go mod tidy
	@go mod download
	@npm --prefix executions install
	@echo "${GREEN}Development environment ready!${NC}"

test: unit integration ## Run all tests
	@echo "${GREEN}All tests completed!${NC}"

unit: ## Run unit tests
	@echo "${GREEN}Running unit tests...${NC}"
	@go test -v -race -cover -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "${GREEN}Unit tests complete! Coverage report: coverage.html${NC}"

integration: ## Run integration tests
	@echo "${GREEN}Running integration tests...${NC}"
	@go test -v -race -tags=integration ./test/integration/...

e2e: ## Run end-to-end tests
	@echo "${GREEN}Running end-to-end tests...${NC}"
	@go test -v -tags=e2e ./test/e2e/...

# =============================================================================
# Docker Operations
# =============================================================================

docker-up: ## Start all Docker services
	@echo "${GREEN}Starting Docker services for testing...${NC}"
	@docker-compose -f docker-compose.arm64.yml up -d
	@sleep 10
	@echo "${GREEN}All services started!${NC}"
	@echo "${YELLOW}MongoDB: localhost:27017${NC}"
	@echo "${YELLOW}Redis: localhost:6379${NC}"
	@echo "${YELLOW}MinIO: localhost:9000${NC}"
	@echo "${YELLOW}HTTPBin: localhost:8080${NC}"

docker-down: ## Stop all Docker services
	@echo "${YELLOW}Stopping Docker services...${NC}"
	@docker-compose -f docker-compose.arm64.yml down
	@echo "${GREEN}All services stopped!${NC}"

docker-logs: ## View Docker container logs
	@docker-compose -f docker-compose.arm64.yml logs -f

clean: docker-down ## Clean up everything
	@echo "${YELLOW}Cleaning up...${NC}"
	@rm -rf $(BUILD_DIR)/
	@rm -rf coverage.*
	@rm -rf *.log
	@rm -rf /tmp/agent_*
	@rm -rf /tmp/recordings
	@docker system prune -f
	@echo "${GREEN}Cleanup complete!${NC}"

# =============================================================================
# Quality & CI/CD
# =============================================================================

ci: check test ## Run CI pipeline
	@echo "${GREEN}CI pipeline complete!${NC}"

all: check build test ## Run all quality checks, build, and test

