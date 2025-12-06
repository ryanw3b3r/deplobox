.PHONY: build test clean install run lint fmt cross-compile help

BINARY_NAME=deplobox
INSTALL_PATH=/usr/local/bin
GO=go
GOFMT=gofmt
GOFLAGS=-ldflags="-s -w"

# Default target
all: build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(GOFLAGS) -o $(BINARY_NAME) cmd/deplobox/main.go
	@echo "Build complete: $(BINARY_NAME)"

# Run tests
test:
	@echo "Running tests..."
	$(GO) test -v -cover ./...

# Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	rm -f deplobox-linux-amd64
	rm -f deplobox-linux-arm64
	rm -f coverage.out coverage.html
	rm -f deployments.db deployments.log
	@echo "Clean complete"

# Install binary to system
install: build
	@echo "Installing to $(INSTALL_PATH)..."
	sudo cp $(BINARY_NAME) $(INSTALL_PATH)/
	sudo chmod +x $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Installation complete"

# Uninstall binary from system
uninstall:
	@echo "Uninstalling from $(INSTALL_PATH)..."
	sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Uninstall complete"

# Run the application
run:
	@echo "Running $(BINARY_NAME)..."
	$(GO) run cmd/deplobox/main.go -config ./projects.yaml

# Cross-compile for Linux
cross-compile:
	@echo "Cross-compiling for Linux AMD64..."
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o deplobox-linux-amd64 cmd/deplobox/main.go
	@echo "Cross-compiling for Linux ARM64..."
	GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -o deplobox-linux-arm64 cmd/deplobox/main.go
	@echo "Cross-compilation complete"
	@ls -lh deplobox-linux-*

# Lint code
lint:
	@echo "Running linter..."
	$(GO) vet ./...
	@echo "Lint complete"

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .
	@echo "Format complete"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GO) mod download
	$(GO) mod tidy
	@echo "Dependencies updated"

# Help
help:
	@echo "Available targets:"
	@echo "  make build          - Build the binary"
	@echo "  make test           - Run tests"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make install        - Install binary to $(INSTALL_PATH)"
	@echo "  make uninstall      - Uninstall binary from $(INSTALL_PATH)"
	@echo "  make run            - Run the application"
	@echo "  make cross-compile  - Cross-compile for Linux (AMD64 and ARM64)"
	@echo "  make lint           - Run linter"
	@echo "  make fmt            - Format code"
	@echo "  make deps           - Download and update dependencies"
	@echo "  make help           - Show this help message"
