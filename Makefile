# MongoDB Exporter Makefile

# Variables
BINARY_NAME=mongo-exporter
VERSION?=1.0.0
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD)
LDFLAGS=-ldflags "-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT} -w -s"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Docker parameters
DOCKER_IMAGE=mongo-exporter
DOCKER_TAG=latest

.PHONY: all build clean test coverage deps lint docker-build docker-run help

# Default target
all: clean deps test build

# Build the application
build:
	@echo "Building ${BINARY_NAME}..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) main.go

# Build optimized version
build-optimized:
	@echo "Building optimized ${BINARY_NAME}..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -a -installsuffix cgo -o $(BINARY_NAME) main.go

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f coverage.out
	rm -rf release/

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

# Install dependencies for development
dev-deps:
	@echo "Installing development dependencies..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run the application
run: build
	@echo "Running ${BINARY_NAME}..."
	./$(BINARY_NAME) -config config.yaml

# Run with environment variables
run-env: build
	@echo "Running ${BINARY_NAME} with environment variables..."
	MONGO_URI=mongodb://localhost:27017 ./$(BINARY_NAME)

# Docker build
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Docker run
docker-run: docker-build
	@echo "Running Docker container..."
	docker run -p 8080:8080 $(DOCKER_IMAGE):$(DOCKER_TAG)

# Docker run with custom config
docker-run-config: docker-build
	@echo "Running Docker container with custom config..."
	docker run -p 8080:8080 -v $(PWD)/config.yaml:/app/config.yaml $(DOCKER_IMAGE):$(DOCKER_TAG)

# Docker run with environment variables
docker-run-env: docker-build
	@echo "Running Docker container with environment variables..."
	docker run -p 8080:8080 -e MONGO_URI=mongodb://host.docker.internal:27017 $(DOCKER_IMAGE):$(DOCKER_TAG)

# Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

# Vet code
vet:
	@echo "Vetting code..."
	$(GOCMD) vet ./...

# Generate mocks (if using mockery)
mocks:
	@echo "Generating mocks..."
	mockery --all

# Install the application
install: build
	@echo "Installing ${BINARY_NAME}..."
	cp $(BINARY_NAME) /usr/local/bin/

# Uninstall the application
uninstall:
	@echo "Uninstalling ${BINARY_NAME}..."
	rm -f /usr/local/bin/$(BINARY_NAME)

# Create release
release: clean deps test build-optimized
	@echo "Creating release..."
	mkdir -p release
	cp $(BINARY_NAME) release/
	cp config.yaml release/
	cp README.md release/
	tar -czf release/$(BINARY_NAME)-$(VERSION).tar.gz -C release .

# Build for multiple platforms
build-multi: clean deps
	@echo "Building for multiple platforms..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o release/$(BINARY_NAME)-linux-amd64 main.go
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o release/$(BINARY_NAME)-linux-arm64 main.go
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o release/$(BINARY_NAME)-darwin-amd64 main.go
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o release/$(BINARY_NAME)-darwin-arm64 main.go
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o release/$(BINARY_NAME)-windows-amd64.exe main.go

# Security scan
security-scan:
	@echo "Running security scan..."
	gosec ./...

# Performance benchmark
benchmark:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

# Show help
help:
	@echo "Available targets:"
	@echo "  all              - Clean, deps, test, and build"
	@echo "  build            - Build the application"
	@echo "  build-optimized  - Build optimized version for production"
	@echo "  build-multi      - Build for multiple platforms"
	@echo "  clean            - Clean build artifacts"
	@echo "  test             - Run tests"
	@echo "  coverage         - Run tests with coverage report"
	@echo "  benchmark        - Run performance benchmarks"
	@echo "  deps             - Download dependencies"
	@echo "  lint             - Run linter"
	@echo "  dev-deps         - Install development dependencies"
	@echo "  run              - Build and run with config file"
	@echo "  run-env          - Build and run with environment variables"
	@echo "  docker-build     - Build Docker image"
	@echo "  docker-run       - Build and run Docker container"
	@echo "  fmt              - Format code"
	@echo "  vet              - Vet code"
	@echo "  security-scan    - Run security scan"
	@echo "  install          - Install to /usr/local/bin"
	@echo "  uninstall        - Remove from /usr/local/bin"
	@echo "  release          - Create release package"
	@echo "  help             - Show this help message" 