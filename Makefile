# Evacuator Makefile

# Variables
BINARY_NAME=evacuator
MAIN_PATH=./cmd/evacuator
DOCKER_IMAGE=rahadiangg/evacuator
VERSION?=dev

# Go variables
GOOS?=$(shell go env GOOS)
GOARCH?=$(shell go env GOARCH)
CGO_ENABLED?=0

# Build flags - simple optimization flags
LDFLAGS=-s -w

.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build the evacuator binary
	@echo "Building $(BINARY_NAME) for $(GOOS)/$(GOARCH)..."
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags "$(LDFLAGS)" \
		-o $(BINARY_NAME) \
		$(MAIN_PATH)

.PHONY: build-linux
build-linux: ## Build Linux binary for containers
	@echo "Building $(BINARY_NAME) for linux/amd64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags "$(LDFLAGS)" \
		-o $(BINARY_NAME) \
		$(MAIN_PATH)

.PHONY: build-all
build-all: ## Build binaries for all platforms
	@echo "Building for multiple platforms..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 make build && mv $(BINARY_NAME) bin/$(BINARY_NAME)-linux-amd64
	GOOS=linux GOARCH=arm64 make build && mv $(BINARY_NAME) bin/$(BINARY_NAME)-linux-arm64
	GOOS=darwin GOARCH=amd64 make build && mv $(BINARY_NAME) bin/$(BINARY_NAME)-darwin-amd64
	GOOS=darwin GOARCH=arm64 make build && mv $(BINARY_NAME) bin/$(BINARY_NAME)-darwin-arm64
	GOOS=windows GOARCH=amd64 make build && mv $(BINARY_NAME) bin/$(BINARY_NAME)-windows-amd64.exe

.PHONY: test
test: ## Run tests
	@echo "Running tests..."
	go test -v -race ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: test-ci
test-ci: ## Run tests for CI (with coverage but no HTML report)
	@echo "Running tests for CI..."
	go test -v -race -coverprofile=coverage.out ./...

.PHONY: lint
lint: ## Run linters (install if needed)
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

.PHONY: fmt
fmt: ## Format Go code and tidy modules
	go fmt ./...
	go mod tidy

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: check
check: fmt vet test ## Run all local checks (format, vet, test)

.PHONY: ci-check
ci-check: fmt vet lint test-ci ## Run all CI checks (format, vet, lint, test with coverage)

.PHONY: clean
clean: ## Clean build artifacts
	rm -f $(BINARY_NAME)
	rm -rf bin/
	rm -f coverage.out coverage.html

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "Building Docker image $(DOCKER_IMAGE):$(VERSION)..."
	docker build -t $(DOCKER_IMAGE):$(VERSION) .
	docker tag $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest

.PHONY: docker-push
docker-push: ## Push Docker image to registry
	@echo "Pushing Docker image $(DOCKER_IMAGE):$(VERSION)..."
	docker push $(DOCKER_IMAGE):$(VERSION)
	docker push $(DOCKER_IMAGE):latest

.PHONY: docker-run
docker-run: ## Run Docker container
	docker run --rm -it \
		-v $(PWD)/config:/etc/evacuator \
		$(DOCKER_IMAGE):$(VERSION)

.PHONY: deps
deps: ## Download and verify dependencies
	go mod download
	go mod verify

.PHONY: deps-update
deps-update: ## Update dependencies
	go get -u ./...
	go mod tidy

.PHONY: dev-setup
dev-setup: deps ## Setup development environment
	@echo "Setting up development environment..."
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Development environment ready!"

.PHONY: release
release: ci-check build-all ## Prepare release (run all checks and build all platforms)
	@echo "Release $(VERSION) prepared!"
	@echo "Binaries available in bin/ directory"

# Kubernetes targets
.PHONY: k8s-apply
k8s-apply: ## Apply Kubernetes manifests
	kubectl apply -f deploy/

.PHONY: k8s-delete
k8s-delete: ## Delete Kubernetes resources
	kubectl delete -f deploy/

.PHONY: k8s-logs
k8s-logs: ## View logs from Kubernetes deployment
	kubectl logs -f -l app=evacuator -n kube-system

# Development and debugging
.PHONY: run
run: build ## Build and run the evacuator locally
	./$(BINARY_NAME)

.PHONY: dev
dev: ## Run in development mode with auto-restart (requires air)
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	air

.PHONY: debug
debug: build ## Run with debug logging
	@echo "Running with debug logging..."
	DRY_RUN=true LOG_LEVEL=debug ./$(BINARY_NAME)

# Default target
.DEFAULT_GOAL := help
