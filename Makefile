.PHONY: build test clean install run lint fmt cross-compile help dist dist-clean version-info

BINARY_NAME=deplobox
INSTALL_PATH=/usr/local/bin
GO=go
GOFMT=gofmt

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Build flags with version info
LDFLAGS := -s -w \
           -X 'main.version=$(VERSION)' \
           -X 'main.gitCommit=$(GIT_COMMIT)' \
           -X 'main.buildDate=$(BUILD_DATE)'

GOFLAGS=-ldflags="$(LDFLAGS)"

# Distribution
DIST_DIR=dist
PLATFORMS=macos linux-arm64 linux-amd64

# Platform-specific build settings
MACOS_GOOS=darwin
MACOS_GOARCH=arm64
LINUX_ARM64_GOOS=linux
LINUX_ARM64_GOARCH=arm64
LINUX_AMD64_GOOS=linux
LINUX_AMD64_GOARCH=amd64

# Default target
all: build

# Build for current platform only
build:
	@echo "Building $(BINARY_NAME) for current platform..."
	$(GO) build $(GOFLAGS) -o $(BINARY_NAME) ./cmd/deplobox
	@echo "Build complete: $(BINARY_NAME)"
	@echo ""
	@./$(BINARY_NAME) version

# Build distribution packages for all platforms
dist: dist-clean
	@echo "Building distribution packages for all platforms..."
	@mkdir -p $(DIST_DIR)

	# Build for macOS (ARM64)
	@echo "Building for macOS ARM64..."
	@mkdir -p $(DIST_DIR)/macos/config
	@mkdir -p $(DIST_DIR)/macos/templates
	GOOS=$(MACOS_GOOS) GOARCH=$(MACOS_GOARCH) $(GO) build $(GOFLAGS) -o $(DIST_DIR)/macos/$(BINARY_NAME) ./cmd/deplobox
	cp config/projects.example.yaml $(DIST_DIR)/macos/config/
	cp templates/nginx-site.template $(DIST_DIR)/macos/templates/
	cp templates/nginx-laravel-site.template $(DIST_DIR)/macos/templates/
	cp templates/systemd-service.template $(DIST_DIR)/macos/templates/

	# Build for Linux ARM64
	@echo "Building for Linux ARM64..."
	@mkdir -p $(DIST_DIR)/linux-arm64/config
	@mkdir -p $(DIST_DIR)/linux-arm64/templates
	GOOS=$(LINUX_ARM64_GOOS) GOARCH=$(LINUX_ARM64_GOARCH) $(GO) build $(GOFLAGS) -o $(DIST_DIR)/linux-arm64/$(BINARY_NAME) ./cmd/deplobox
	cp config/projects.example.yaml $(DIST_DIR)/linux-arm64/config/
	cp templates/nginx-site.template $(DIST_DIR)/linux-arm64/templates/
	cp templates/nginx-laravel-site.template $(DIST_DIR)/linux-arm64/templates/
	cp templates/systemd-service.template $(DIST_DIR)/linux-arm64/templates/

	# Build for Linux AMD64
	@echo "Building for Linux AMD64..."
	@mkdir -p $(DIST_DIR)/linux-amd64/config
	@mkdir -p $(DIST_DIR)/linux-amd64/templates
	GOOS=$(LINUX_AMD64_GOOS) GOARCH=$(LINUX_AMD64_GOARCH) $(GO) build $(GOFLAGS) -o $(DIST_DIR)/linux-amd64/$(BINARY_NAME) ./cmd/deplobox
	cp config/projects.example.yaml $(DIST_DIR)/linux-amd64/config/
	cp templates/nginx-site.template $(DIST_DIR)/linux-amd64/templates/
	cp templates/nginx-laravel-site.template $(DIST_DIR)/linux-amd64/templates/
	cp templates/systemd-service.template $(DIST_DIR)/linux-amd64/templates/

	# Create archives
	@echo "Creating distribution archives..."
	cd $(DIST_DIR)/macos && zip -r ../deplobox-macos.zip . && cd ../..
	cd $(DIST_DIR)/linux-arm64 && zip -r ../deplobox-linux-arm64.zip . && cd ../..
	cd $(DIST_DIR)/linux-amd64 && zip -r ../deplobox-linux-amd64.zip . && cd ../..

	@echo "Distribution build complete!"
	@echo ""
	@echo "Distribution structure:"
	@tree -L 3 $(DIST_DIR) 2>/dev/null || find $(DIST_DIR) -type f -o -type d | head -30
	@echo ""
	@echo "Archives created:"
	@ls -lh $(DIST_DIR)/*.zip $(DIST_DIR)/*.zip 2>/dev/null || true

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
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	rm -f deplobox-linux-amd64
	rm -f deplobox-linux-arm64
	rm -f coverage.out coverage.html
	rm -f deployments.db deployments.log
	@echo "Clean complete"

# Clean distribution directory
dist-clean:
	@echo "Cleaning distribution directory..."
	rm -rf $(DIST_DIR)
	@echo "Distribution clean complete"

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
	@echo "Running $(BINARY_NAME) serve..."
	$(GO) run ./cmd/deplobox serve --config ./projects.yaml

# Cross-compile for Linux (legacy - use 'make dist' instead)
cross-compile:
	@echo "Cross-compiling for Linux AMD64..."
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o deplobox-linux-amd64 ./cmd/deplobox
	@echo "Cross-compiling for Linux ARM64..."
	GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -o deplobox-linux-arm64 ./cmd/deplobox
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

# Show version info
version-info:
	@echo "Version:    $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"

# Help
help:
	@echo "Available targets:"
	@echo "  make build          - Build binaries for current platform"
	@echo "  make dist           - Build distribution packages for all platforms (macOS, Linux ARM64, Linux AMD64)"
	@echo "  make test           - Run tests"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make dist-clean     - Clean distribution directory"
	@echo "  make install        - Install binary to $(INSTALL_PATH)"
	@echo "  make uninstall      - Uninstall binary from $(INSTALL_PATH)"
	@echo "  make run            - Run the application"
	@echo "  make cross-compile  - Cross-compile for Linux (legacy)"
	@echo "  make lint           - Run linter"
	@echo "  make fmt            - Format code"
	@echo "  make deps           - Download and update dependencies"
	@echo "  make help           - Show this help message"
