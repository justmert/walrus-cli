# Walrus CLI Makefile
# Run 'make help' for a list of available targets

.PHONY: help
help: ## Show this help message
	@echo "Walrus CLI - Development Makefile"
	@echo "================================="
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Variables
BINARY_NAME=walrus-cli
MAIN_PATH=cmd/walrus-cli
WEB_PATH=web/walrus-ui
BUILD_DIR=dist
VERSION=$(shell git describe --tags --always --dirty)
COMMIT=$(shell git rev-parse --short HEAD)
DATE=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"
GO_FILES=$(shell find . -name '*.go' -not -path './vendor/*' -not -path './.git/*')

.PHONY: all
all: clean test build ## Clean, test and build

.PHONY: build
build: web-build ## Build the CLI binary with embedded web UI
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -tags embed -o $(BUILD_DIR)/$(BINARY_NAME) ./$(MAIN_PATH)
	@echo "✓ Built $(BUILD_DIR)/$(BINARY_NAME)"

.PHONY: build-all
build-all: web-build ## Build for all platforms
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	# Linux AMD64
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -tags embed -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./$(MAIN_PATH)
	# Linux ARM64
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -tags embed -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./$(MAIN_PATH)
	# macOS AMD64
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -tags embed -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./$(MAIN_PATH)
	# macOS ARM64 (M1)
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -tags embed -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./$(MAIN_PATH)
	# Windows AMD64
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -tags embed -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./$(MAIN_PATH)
	@echo "✓ Built all platforms"

.PHONY: web-build
web-build: ## Build the web UI
	@echo "Building web UI..."
	@cd $(WEB_PATH) && npm install --silent && npm run build
	@rm -rf $(MAIN_PATH)/web_dist
	@cp -r $(WEB_PATH)/dist $(MAIN_PATH)/web_dist
	@echo "✓ Web UI built"

.PHONY: web-dev
web-dev: ## Run web UI in development mode
	@cd $(WEB_PATH) && npm run dev

.PHONY: install
install: build ## Install the binary to GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
	@echo "✓ Installed to $(GOPATH)/bin/$(BINARY_NAME)"

.PHONY: install-local
install-local: build ## Install the binary to ~/bin
	@echo "Installing $(BINARY_NAME) to ~/bin..."
	@mkdir -p ~/bin
	@cp $(BUILD_DIR)/$(BINARY_NAME) ~/bin/$(BINARY_NAME)
	@echo "✓ Installed to ~/bin/$(BINARY_NAME)"

.PHONY: test
test: ## Run tests
	@echo "Running tests..."
	@go test -v -race -cover ./...
	@echo "✓ Tests passed"

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report generated: coverage.html"

.PHONY: lint
lint: ## Run linters
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with: brew install golangci-lint"; \
		exit 1; \
	fi
	@echo "✓ Linting passed"

.PHONY: fmt
fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@cd $(WEB_PATH) && npm run lint --fix 2>/dev/null || true
	@echo "✓ Code formatted"

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@rm -f $(BINARY_NAME)
	@rm -rf $(MAIN_PATH)/web_dist
	@echo "✓ Cleaned"

.PHONY: deps
deps: ## Download and verify dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod verify
	@cd $(WEB_PATH) && npm install
	@echo "✓ Dependencies installed"

.PHONY: tidy
tidy: ## Tidy up modules
	@echo "Tidying modules..."
	@go mod tidy
	@echo "✓ Modules tidied"

.PHONY: run
run: ## Run the CLI locally
	@go run ./$(MAIN_PATH)

.PHONY: run-web
run-web: ## Run the web command
	@go run ./$(MAIN_PATH) web

.PHONY: release-dry
release-dry: ## Dry run of release process
	@echo "Running release dry run..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser release --snapshot --clean; \
	else \
		echo "goreleaser not installed. Install with: brew install goreleaser"; \
		exit 1; \
	fi

.PHONY: release
release: ## Create a new release (requires tag)
	@echo "Creating release..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser release --clean; \
	else \
		echo "goreleaser not installed. Install with: brew install goreleaser"; \
		exit 1; \
	fi

.PHONY: tag
tag: ## Create a new version tag (usage: make tag VERSION=v1.0.0)
	@if [ -z "$(VERSION)" ]; then \
		echo "VERSION is not set. Usage: make tag VERSION=v1.0.0"; \
		exit 1; \
	fi
	@echo "Creating tag $(VERSION)..."
	@git tag -a $(VERSION) -m "Release $(VERSION)"
	@echo "✓ Tag $(VERSION) created. Push with: git push origin $(VERSION)"

.PHONY: dev
dev: ## Start development environment (CLI + Web in background)
	@echo "Starting development environment..."
	@make build
	@$(BUILD_DIR)/$(BINARY_NAME) web --background &
	@echo "✓ Development environment started"
	@echo "  Web UI: http://localhost:5173"
	@echo "  API: http://localhost:3002"
	@echo "  Stop with: walrus-cli stop"

.PHONY: stop
stop: ## Stop background services
	@echo "Stopping background services..."
	@$(BUILD_DIR)/$(BINARY_NAME) stop 2>/dev/null || walrus-cli stop 2>/dev/null || true
	@echo "✓ Services stopped"

# Docker targets (future use)
.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t $(BINARY_NAME):$(VERSION) .
	@echo "✓ Docker image built"

.PHONY: docker-run
docker-run: ## Run Docker container
	@docker run -it --rm -p 3002:3002 -p 5173:5173 $(BINARY_NAME):$(VERSION)