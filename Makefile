# Evacuator Makefile

# Variables
BINARY_NAME=evacuator
MAIN_PATH=./cmd/evacuator
DOCKER_IMAGE=rahadiangg/evacuator
DOCKER_EXTRA_PLATFORM=linux/arm64,linux/amd64
VERSION?=latest

# Go variables
GOOS?=$(shell go env GOOS)
GOARCH?=$(shell go env GOARCH)
CGO_ENABLED?=1

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

.PHONY: deps
deps: ## Download and verify dependencies
	go mod download
	go mod verify

# Development and debugging
.PHONY: run
run: build ## Build and run the evacuator locally
	./$(BINARY_NAME)

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "Building Docker image $(DOCKER_IMAGE):$(VERSION)..."
	docker buildx build -t $(DOCKER_IMAGE):$(VERSION) --platform $(DOCKER_EXTRA_PLATFORM) .

.PHONY: docker-push
docker-push: ## Push Docker image to registry
	@echo "Pushing Docker image $(DOCKER_IMAGE):$(VERSION)..."
	docker push $(DOCKER_IMAGE):$(VERSION)

.PHONY: docker-run
docker-run: ## Run Docker container
	docker run --rm -it \
		$(DOCKER_IMAGE):$(VERSION)

# Default target
.DEFAULT_GOAL := help
