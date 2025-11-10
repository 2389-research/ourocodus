.PHONY: all build test smoke-relay smoke-session smoke-all test-e2e demo interactive run stop clean lint fmt check pre-commit nats-start nats-stop nats-logs nats-health assets assets-check tailwind-download

# Default target: build and test
all: build test

# Compile and bundle web assets (TypeScript, CSS)
# This runs before build to prepare assets for embedding
assets:
	@echo "Compiling web assets..."
	@# TypeScript/JavaScript bundling
	@if [ -d "internal/webapp/src" ]; then \
		echo "  → Bundling TypeScript with esbuild..."; \
		mise exec -- esbuild internal/webapp/src/app.ts \
			--bundle \
			--minify \
			--target=es2020 \
			--format=iife \
			--outfile=internal/webapp/web/app.js; \
		echo "  → Bundling logger.ts..."; \
		mise exec -- esbuild internal/webapp/src/logger.ts \
			--bundle \
			--minify \
			--target=es2020 \
			--format=iife \
			--outfile=internal/webapp/web/logger.js; \
	fi
	@# Tailwind CSS compilation
	@if [ -f "bin/tailwindcss" ] && [ -f "internal/webapp/web/tailwind.input.css" ]; then \
		echo "  → Compiling Tailwind CSS..."; \
		cd internal/webapp && ../../bin/tailwindcss -i web/tailwind.input.css -o web/tailwind.css --minify; \
	fi
	@# CSS minification (for non-Tailwind CSS)
	@if [ -f "internal/webapp/web/styles.css" ]; then \
		echo "  → Minifying CSS..."; \
		mise exec -- minify -o internal/webapp/web/styles.min.css internal/webapp/web/styles.css 2>/dev/null || true; \
	fi
	@# Generate content-hash fingerprinted assets
	@echo "  → Generating asset fingerprints..."
	@go run internal/webapp/tools/asset-hash.go internal/webapp/web internal/webapp/web/asset-manifest.json
	@# Inject hashed filenames into HTML
	@echo "  → Injecting hashes into HTML..."
	@go run internal/webapp/tools/inject-hashes.go internal/webapp/web/asset-manifest.json internal/webapp/web/index.html
	@if [ -f "internal/webapp/web/protocol-inspector.html" ]; then \
		go run internal/webapp/tools/inject-hashes.go internal/webapp/web/asset-manifest.json internal/webapp/web/protocol-inspector.html; \
	fi
	@echo "✓ Assets compiled"

# Check if asset tools are installed
assets-check:
	@echo "Checking asset compilation tools..."
	@mise exec -- esbuild --version >/dev/null 2>&1 && echo "  ✓ esbuild installed" || echo "  ✗ esbuild missing (run: mise install)"
	@mise exec -- minify --version >/dev/null 2>&1 && echo "  ✓ minify installed" || echo "  ✗ minify missing (run: mise install)"
	@[ -f "bin/tailwindcss" ] && echo "  ✓ tailwindcss installed" || echo "  ✗ tailwindcss missing (run: make tailwind-download)"

# Download Tailwind CSS standalone CLI
tailwind-download:
	@echo "Downloading Tailwind CSS standalone CLI..."
	@mkdir -p bin
	@# Detect OS and download appropriate binary
	@if [ "$$(uname)" = "Darwin" ]; then \
		if [ "$$(uname -m)" = "arm64" ]; then \
			echo "  → Downloading for macOS ARM64..."; \
			curl -sL https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-macos-arm64 -o bin/tailwindcss; \
		else \
			echo "  → Downloading for macOS x64..."; \
			curl -sL https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-macos-x64 -o bin/tailwindcss; \
		fi \
	elif [ "$$(uname)" = "Linux" ]; then \
		if [ "$$(uname -m)" = "aarch64" ]; then \
			echo "  → Downloading for Linux ARM64..."; \
			curl -sL https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-arm64 -o bin/tailwindcss; \
		else \
			echo "  → Downloading for Linux x64..."; \
			curl -sL https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64 -o bin/tailwindcss; \
		fi \
	else \
		echo "  ✗ Unsupported OS: $$(uname)"; \
		exit 1; \
	fi
	@chmod +x bin/tailwindcss
	@echo "✓ Tailwind CSS CLI installed to bin/tailwindcss"

# Build all binaries (with asset compilation)
build: assets
	@echo "Building binaries..."
	@mkdir -p bin
	go build -o bin/relay ./cmd/relay
	go build -o bin/cli ./cmd/cli
	go build -o bin/echo-agent ./cmd/echo-agent
	go build -o bin/event-logger ./cmd/event-logger
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

# Run end-to-end integration tests
test-e2e:
	@echo "Running E2E integration tests..."
	@if [ -z "$$ANTHROPIC_API_KEY" ]; then \
		echo "Error: ANTHROPIC_API_KEY environment variable is not set"; \
		echo "Please set your API key: export ANTHROPIC_API_KEY='your-key'"; \
		exit 1; \
	fi
	@cd tests/e2e && go test -v -timeout $${E2E_TEST_TIMEOUT:-10m} .

# Run interactive demo showcasing PR #27 features
demo:
	@echo "Running PR #27 features demo..."
	go run examples/basic-demo/main.go

# Run interactive REPL for manual testing
interactive:
	@echo "Starting interactive REPL..."
	go run examples/interactive-repl/main.go

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

# Start NATS server with JetStream
nats-start:
	@echo "Starting NATS server with JetStream..."
	@docker-compose up -d nats
	@echo "Waiting for NATS to be healthy..."
	@timeout 30 sh -c 'until docker-compose ps nats | grep -q "healthy"; do sleep 1; done' || (echo "NATS failed to become healthy" && exit 1)
	@echo "Running NATS initialization..."
	@docker-compose up nats-init
	@echo "Starting Prometheus exporter..."
	@docker-compose up -d nats-exporter
	@echo ""
	@echo "✓ NATS server started successfully"
	@echo ""
	@echo "Endpoints:"
	@echo "  - NATS client: nats://localhost:4222"
	@echo "  - HTTP monitoring: http://localhost:8222"
	@echo "  - Prometheus metrics: http://localhost:7777/metrics"
	@echo ""
	@echo "Verify with: make nats-health"

# Stop NATS server
nats-stop:
	@echo "Stopping NATS server..."
	@docker-compose stop nats nats-init nats-exporter
	@echo "✓ NATS server stopped"

# View NATS server logs
nats-logs:
	@docker-compose logs -f nats

# Check NATS server health
nats-health:
	@echo "Checking NATS server health..."
	@echo ""
	@echo "=== Health Check ==="
	@curl -sf http://localhost:8222/healthz && echo "✓ Health endpoint OK" || echo "✗ Health endpoint failed"
	@echo ""
	@echo "=== Server Info ==="
	@curl -s http://localhost:8222/varz | jq -r '"Version: " + .version, "Uptime: " + .uptime, "Connections: " + (.connections | tostring), "JetStream: " + (if .jetstream.config.max_memory > 0 then "enabled" else "disabled" end)'
	@echo ""
	@echo "=== JetStream Streams ==="
	@docker-compose exec -T nats nats stream list 2>/dev/null || echo "Note: Install nats CLI to list streams (see docs/NATS.md)"
