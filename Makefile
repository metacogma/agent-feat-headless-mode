# nkk: Comprehensive Makefile for testing and deployment
# Based on Google's build practices

.PHONY: help test unit integration e2e clean setup docker-up docker-down

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[1;33m
NC := \033[0m # No Color

help: ## Show this help message
	@echo '${GREEN}Available targets:${NC}'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# =============================================================================
# Environment Setup
# =============================================================================

setup: ## Setup development environment
	@echo "${GREEN}Setting up development environment...${NC}"
	@go mod download
	@go install github.com/golang/mock/mockgen@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@cp .env.test .env.test.local
	@echo "${GREEN}Setup complete!${NC}"

docker-up: ## Start all test dependencies in Docker
	@echo "${GREEN}Starting Docker containers...${NC}"
	@docker-compose -f docker-compose.test.yml up -d
	@echo "${YELLOW}Waiting for services to be healthy...${NC}"
	@sleep 10
	@docker-compose -f docker-compose.test.yml ps
	@echo "${GREEN}All services are up!${NC}"

docker-down: ## Stop all Docker containers
	@echo "${YELLOW}Stopping Docker containers...${NC}"
	@docker-compose -f docker-compose.test.yml down -v
	@echo "${GREEN}Containers stopped!${NC}"

docker-logs: ## View Docker container logs
	@docker-compose -f docker-compose.test.yml logs -f

# =============================================================================
# Testing
# =============================================================================

test: docker-up unit integration e2e docker-down ## Run all tests
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

benchmark: ## Run performance benchmarks
	@echo "${GREEN}Running benchmarks...${NC}"
	@go test -bench=. -benchmem ./...

# =============================================================================
# Code Quality
# =============================================================================

lint: ## Run linters
	@echo "${GREEN}Running linters...${NC}"
	@golangci-lint run --timeout=5m ./...

fmt: ## Format code
	@echo "${GREEN}Formatting code...${NC}"
	@go fmt ./...
	@goimports -w .

vet: ## Run go vet
	@echo "${GREEN}Running go vet...${NC}"
	@go vet ./...

security: ## Run security scan
	@echo "${GREEN}Running security scan...${NC}"
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@gosec -fmt json -out security-report.json ./...

# =============================================================================
# Build & Run
# =============================================================================

build: ## Build the application
	@echo "${GREEN}Building application...${NC}"
	@go build -o bin/agent ./cmd/agent

run: docker-up ## Run the application with test environment
	@echo "${GREEN}Starting application...${NC}"
	@export $$(cat .env.test | xargs) && go run ./cmd/agent

run-client: ## Run the test client
	@echo "${GREEN}Running test client...${NC}"
	@cd ~/Documents/code_agent_client && go run main.go

# =============================================================================
# Database Operations
# =============================================================================

db-seed: ## Seed test data
	@echo "${GREEN}Seeding test data...${NC}"
	@go run ./test/seed/main.go

db-migrate: ## Run database migrations
	@echo "${GREEN}Running migrations...${NC}"
	@go run ./migrations/migrate.go up

db-rollback: ## Rollback database migrations
	@echo "${YELLOW}Rolling back migrations...${NC}"
	@go run ./migrations/migrate.go down

# =============================================================================
# Monitoring
# =============================================================================

metrics: ## View Prometheus metrics
	@echo "${GREEN}Opening Prometheus...${NC}"
	@open http://localhost:9090

grafana: ## Open Grafana dashboard
	@echo "${GREEN}Opening Grafana...${NC}"
	@open http://localhost:3000

logs: ## View application logs
	@tail -f logs/app.log

# =============================================================================
# Cleanup
# =============================================================================

clean: docker-down ## Clean up everything
	@echo "${YELLOW}Cleaning up...${NC}"
	@rm -rf bin/
	@rm -rf coverage.*
	@rm -rf *.log
	@rm -rf /tmp/agent_*
	@rm -rf /tmp/recordings
	@docker system prune -f
	@echo "${GREEN}Cleanup complete!${NC}"

# =============================================================================
# CI/CD
# =============================================================================

ci: lint vet test ## Run CI pipeline
	@echo "${GREEN}CI pipeline complete!${NC}"

release: ## Create a release
	@echo "${GREEN}Creating release...${NC}"
	@goreleaser release --rm-dist