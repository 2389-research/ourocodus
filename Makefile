.PHONY: build test run stop clean lint fmt check pre-commit

# Build all binaries
build:
	@echo "Building binaries..."
	@mkdir -p bin
	go build -o bin/relay ./cmd/relay
	go build -o bin/cli ./cmd/cli
	go build -o bin/echo-agent ./cmd/echo-agent
	@echo "Build complete. Binaries in bin/"

# Run tests
test:
	@echo "Running tests..."
	go test ./...

# Start the system (placeholder for now)
run:
	@echo "Starting system..."
	@echo "Note: Full system startup not yet implemented"
	./bin/relay

# Stop the system (placeholder for now)
stop:
	@echo "Stopping system..."
	@pkill -f "bin/relay" || true
	@pkill -f "bin/echo-agent" || true
	@echo "System stopped"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	go clean
	@echo "Clean complete"

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo >&2 "golangci-lint not installed. Install via mise or manually."; exit 1; }
	golangci-lint run --timeout=5m

# Format code (requires gofumpt)
fmt:
	@echo "Formatting code..."
	@command -v gofumpt >/dev/null 2>&1 || { echo >&2 "gofumpt not installed. Install via mise or manually."; exit 1; }
	gofumpt -l -w .

# Run static analysis (requires staticcheck)
check:
	@echo "Running static analysis..."
	@command -v staticcheck >/dev/null 2>&1 || { echo >&2 "staticcheck not installed. Install via mise or manually."; exit 1; }
	staticcheck ./...

# Run all pre-commit checks
pre-commit: fmt
	@echo "Running all pre-commit checks..."
	go vet ./...
	$(MAKE) lint
	go mod tidy
	$(MAKE) build
	$(MAKE) test
	@echo "All checks passed!"
