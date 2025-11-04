# Suggested Commands for Development

This file contains the most important commands for developing code in Ourocodus.

## Tool Installation

```bash
# Install all development tools (first time setup)
mise install

# Verify tool versions
mise list
```

## Building

```bash
# Build all binaries (relay, cli, echo-agent)
make build
# or
mise run build

# Clean build artifacts
make clean
```

## Testing

```bash
# Run all unit tests
make test
# or
mise run test

# Run with verbose output
go test -v ./...

# Run tests for specific package
go test ./pkg/relay/...

# Run integration tests (requires build tags)
go test -tags=integration ./pkg/agent/packnplay/... -v

# Run end-to-end tests (requires ANTHROPIC_API_KEY)
make test-e2e
# or
./scripts/run-e2e.sh

# Run smoke tests
mise run smoke
# or
./scripts/smoke-test.sh all

# Run smoke tests with fuzzing
./scripts/smoke-test.sh all --fuzz 100
```

## Code Quality

```bash
# Format code with gofumpt (stricter than gofmt)
make fmt
# or
mise run fmt
# or
gofumpt -l -w .

# Run linter
make lint
# or
mise run lint
# or
golangci-lint run --timeout=5m

# Auto-fix linting issues
golangci-lint run --fix

# Run static analysis
make check
# or
mise run check
# or
staticcheck ./...

# Run basic static analysis
go vet ./...

# Clean up dependencies
go mod tidy
```

## Pre-commit Checks

```bash
# Run all pre-commit checks (recommended before committing)
make pre-commit
# or
mise run pre-commit

# This runs: fmt, vet, lint, tidy, build, test
```

## Running the System

```bash
# Start relay server
make run

# Stop system
make stop

# Interactive REPL demo (no API key needed)
make interactive
# or
mise run interactive

# Automated demo (no API key needed)
make demo
# or
mise run demo
```

## NATS Message Bus

```bash
# Start NATS server with JetStream
make nats-start

# Check NATS health
make nats-health

# View NATS logs
make nats-logs

# Stop NATS server
make nats-stop

# NATS endpoints:
# - Client: nats://localhost:4222
# - HTTP Monitoring: http://localhost:8222
# - Prometheus Metrics: http://localhost:7777/metrics
```

## Git and Version Control

```bash
# Standard Darwin/macOS commands
git status
git add .
git commit -m "message"
git push
git pull
git log --oneline -10

# Setup git worktrees
./scripts/setup-worktrees.sh
```

## Pre-commit Hooks (Optional)

```bash
# Install pre-commit (if not already installed)
pip install pre-commit
# or
brew install pre-commit

# Install hooks
pre-commit install

# Run manually on all files
pre-commit run --all-files
```

## Integration Test Cleanup

```bash
# List Packnplay-managed containers
docker ps -a --filter "label=managed-by=packnplay"

# Remove all Packnplay containers
docker ps -a --filter "label=managed-by=packnplay" -q | xargs docker rm -f

# Remove associated worktrees (if needed)
rm -rf ~/.local/share/packnplay/worktrees
```

## Useful Development Commands

```bash
# List files/directories
ls -la

# Find files
find . -name "*.go" -type f

# Search in files
grep -r "pattern" .

# View file contents
cat filename
less filename

# Change directory
cd path/to/directory

# Current directory
pwd
```
