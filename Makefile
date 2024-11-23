# Project configuration
CLI_NAME := s3fs
BUILD_DIR := bin
CLI_SRC := ./

# Docker configuration
DOCKER_REPO := ghcr.io/yindia
VERSION := $(shell git describe --tags --always --dirty)
DOCKER_CLI_NAME := s3fs-cli

# ANSI color codes for prettier output
NO_COLOR := \033[0m
OK_COLOR := \033[32;01m
ERROR_COLOR := \033[31;01m
WARN_COLOR := \033[33;01m

# Declare phony targets (targets that don't represent files)
.PHONY: all bootstrap deps check-go build test docker-build docker-push helm-template helm-lint helm-fmt helm-install helm helm-dep-update

# Default target: run deps, tests, and build
all: deps test build

# Install all dependencies
deps: deps-go 

# Install Go dependencies
deps-go: check-go test
	buf format idl 
	buf generate idl
	go mod download
	go fmt ./...
	go generate ./...

# Check if Go is installed
check-go:
	@which go > /dev/null || (echo "$(ERROR_COLOR)Go is not installed$(NO_COLOR)" && exit 1)

# CLI targets
build-cli: deps-go
	@echo "$(OK_COLOR)==> Building the CLI...$(NO_COLOR)"
	@CGO_ENABLED=0 go build -v -ldflags="-s -w" -o "$(BUILD_DIR)/$(CLI_NAME)" "$(CLI_SRC)"

run-cli: build-cli
	@echo "$(OK_COLOR)==> Running the CLI...$(NO_COLOR)"
	@$(BUILD_DIR)/$(CLI_NAME) --help

docker-build-cli:
	@echo "$(OK_COLOR)==> Building Docker image for CLI...$(NO_COLOR)"
	docker build -t $(DOCKER_REPO)/$(DOCKER_CLI_NAME):$(VERSION) .

docker-push-cli: docker-build-cli
	@echo "$(OK_COLOR)==> Pushing Docker image for CLI...$(NO_COLOR)"
	docker push $(DOCKER_REPO)/$(DOCKER_CLI_NAME):$(VERSION)


# Test targets
test: deps
	@echo "$(OK_COLOR)==> Running the unit tests$(NO_COLOR)"
	@go test -race -tags unit -cover ./...

# Combined targets
build: build-cli 
docker-build: docker-build-cli 
docker-push: docker-push-cli 

# Helm targets
helm-template:
	@echo "$(OK_COLOR)==> Generating Helm templates...$(NO_COLOR)"
	helm template charts/s3fs

helm-lint:
	@echo "$(OK_COLOR)==> Linting Helm charts...$(NO_COLOR)"
	helm lint charts/s3fs

helm-fmt:
	@echo "$(OK_COLOR)==> Formatting Helm charts...$(NO_COLOR)"
	helm lint --strict charts/s3fs

helm-docs:
	@echo "$(OK_COLOR)==> Generating Helm charts README.md...$(NO_COLOR)"
	go install github.com/norwoodj/helm-docs/cmd/helm-docs@latest
	helm-docs -c  ./charts/s3fs/ 

helm-install:
	@echo "$(OK_COLOR)==> Installing Helm charts...$(NO_COLOR)"
	helm install my-release charts/s3fs

helm-dep-update:
	@echo "$(OK_COLOR)==> Updating Helm dependencies...$(NO_COLOR)"
	helm dependency update ./charts/s3fs/

# Run all Helm-related tasks
helm: helm-dep-update helm-template helm-lint helm-fmt helm-docs
	@echo "$(OK_COLOR)==> Helm template, lint, and format completed.$(NO_COLOR)"

# Set up development environment
bootstrap:
	curl -fsSL https://pixi.sh/install.sh | bash
	brew install bufbuild/buf/buf
	brew install mockery
	pixi shell