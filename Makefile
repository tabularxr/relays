.PHONY: build test test-unit test-integration run clean docker-build docker-up docker-down deps lint

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=relay
BINARY_PATH=./cmd/relay

# Build the application
build:
	CGO_ENABLED=1 $(GOBUILD) -o $(BINARY_NAME) $(BINARY_PATH)

# Run tests
test: test-unit test-integration

# Run unit tests
test-unit:
	$(GOTEST) -v ./tests/unit/...

# Run integration tests
test-integration:
	$(GOTEST) -v ./tests/integration/...

# Run the application
run: build
	./$(BINARY_NAME)

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

# Install dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Lint code
lint:
	golangci-lint run

# Docker commands
docker-build:
	docker build -f docker/Dockerfile -t tabular-relay .

docker-up:
	docker-compose -f docker/docker-compose.yml up -d

docker-down:
	docker-compose -f docker/docker-compose.yml down

docker-logs:
	docker-compose -f docker/docker-compose.yml logs -f relay

# Development helpers
dev: deps build test

# Quick test with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Run with race detection
test-race:
	$(GOTEST) -race -v ./...