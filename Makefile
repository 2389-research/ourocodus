.PHONY: build test smoke-relay smoke-session smoke-all run stop clean lint fmt check pre-commit

# Build all binaries
build:
	@echo "Building binaries..."
	@mkdir -p bin
	go build -o bin/relay ./cmd/relay
	go build -o bin/cli ./cmd/cli
	go build -o bin/echo-agent ./cmd/echo-agent
	@echo "Build complete. Binaries in bin/"

# Run unit tests
test:
	@echo "Running unit tests..."
	go test ./...

# Run WebSocket relay integration smoke test
smoke-relay:
	@echo "Running relay integration smoke test..."
	./scripts/smoke-test.sh relay --verbose

# Run session management layer smoke test
smoke-session:
	@echo "Running session management smoke test..."
	./scripts/smoke-test.sh session --verbose

# Run all smoke tests
smoke-all:
	@echo "Running all smoke tests..."
	./scripts/smoke-test.sh all --verbose

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
	mise exec -- golangci-lint run --timeout=5m

# Format code (requires gofumpt)
fmt:
	@echo "Formatting code..."
	mise exec -- gofumpt -l -w .

# Run static analysis (requires staticcheck)
check:
	@echo "Running static analysis..."
	mise exec -- staticcheck ./...

# Run all pre-commit checks
pre-commit: fmt
	@echo "Running all pre-commit checks..."
	go vet ./...
	$(MAKE) lint
	go mod tidy
	$(MAKE) build
	$(MAKE) test
	@echo "All checks passed!"
