.PHONY: help build run test lint clean fmt vet install deps

# Default target
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build the project
	@echo "Building..."
	@go build -v ./...

run: ## Run the example application
	@echo "Running example..."
	@go run examples/main.go

test: ## Run all tests
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...

test-short: ## Run tests without race detector
	@echo "Running tests (short)..."
	@go test -v ./...

coverage: test ## Generate test coverage report
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

lint: ## Run linter (requires golangci-lint)
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install from https://golangci-lint.run/usage/install/"; \
	fi

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@gofmt -s -w .

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

clean: ## Clean build artifacts and cache
	@echo "Cleaning..."
	@go clean -cache -testcache -modcache
	@rm -f coverage.out coverage.html

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

install: ## Install the binary
	@echo "Installing..."
	@go install ./...

check: fmt vet lint test ## Run all checks (fmt, vet, lint, test)

# Deployment tools
build-tools: ## Build deployment tools
	@echo "Building deployment tools..."
	@mkdir -p bin
	@go build -o bin/deploy-cloudrun cmd/deploy/cloudrun/main.go
	@go build -o bin/deploy-k8s cmd/deploy/k8s/main.go
	@echo "Deployment tools built successfully in bin/"

deploy-cloudrun: ## Deploy to Cloud Run (requires GCP_PROJECT_ID)
	@echo "Deploying to Cloud Run..."
	@go run cmd/deploy/cloudrun/main.go \
		-project $(GCP_PROJECT_ID) \
		-region $(or $(GCP_REGION),us-central1) \
		-env $(or $(ENVIRONMENT),production)

deploy-cloudrun-staging: ## Deploy to Cloud Run staging
	@echo "Deploying to Cloud Run staging..."
	@go run cmd/deploy/cloudrun/main.go \
		-project $(GCP_PROJECT_ID) \
		-region $(or $(GCP_REGION),us-central1) \
		-env staging \
		-service aixgo-mcp-staging

deploy-cloudrun-production: ## Deploy to Cloud Run production
	@echo "Deploying to Cloud Run production..."
	@go run cmd/deploy/cloudrun/main.go \
		-project $(GCP_PROJECT_ID) \
		-region $(or $(GCP_REGION),us-central1) \
		-env production

deploy-k8s: ## Deploy to Kubernetes (requires GCP_PROJECT_ID, GKE_CLUSTER)
	@echo "Deploying to Kubernetes..."
	@go run cmd/deploy/k8s/main.go \
		-project $(GCP_PROJECT_ID) \
		-cluster $(or $(GKE_CLUSTER),aixgo-cluster) \
		-zone $(or $(GKE_ZONE),us-central1) \
		-env $(or $(ENVIRONMENT),staging)

deploy-k8s-staging: ## Deploy to Kubernetes staging
	@echo "Deploying to Kubernetes staging..."
	@go run cmd/deploy/k8s/main.go \
		-project $(GCP_PROJECT_ID) \
		-cluster $(or $(GKE_CLUSTER),aixgo-cluster) \
		-zone $(or $(GKE_ZONE),us-central1) \
		-env staging

deploy-k8s-production: ## Deploy to Kubernetes production
	@echo "Deploying to Kubernetes production..."
	@go run cmd/deploy/k8s/main.go \
		-project $(GCP_PROJECT_ID) \
		-cluster $(or $(GKE_CLUSTER),aixgo-cluster) \
		-zone $(or $(GKE_ZONE),us-central1) \
		-env production

# =============================================================================
# Web targets (delegated to web/Makefile)
# =============================================================================

.PHONY: web-dev web-build web-clean web-lint

web-dev: ## Start Hugo development server
	$(MAKE) -C web dev

web-build: ## Build Hugo site for production
	$(MAKE) -C web build

web-clean: ## Clean web build artifacts
	$(MAKE) -C web clean

web-lint: ## Lint web content
	$(MAKE) -C web lint

.DEFAULT_GOAL := help
