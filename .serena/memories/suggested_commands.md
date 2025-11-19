# Suggested Commands for Development

## Essential Commands

### Build
```bash
# Build all binaries (relay, cli, echo-agent, event-logger)
make build

# Or via mise
mise run build

# Output: bin/relay, bin/cli, bin/echo-agent, bin/event-logger
```

### Run System
```bash
# Start relay server
make run

# Build agent Docker image (required before spawning agents)
make agent-image

# Set required API key
export ANTHROPIC_API_KEY=sk-...
```

### Testing
```bash
# Run all Go unit tests
make test
# Or: go test ./...

# Run E2E integration tests (requires ANTHROPIC_API_KEY)
make test-e2e
# Or: ./scripts/run-e2e.sh

# Run smoke tests (session management, WebSocket integration)
mise run smoke
# Or: ./scripts/smoke-test.sh all

# Smoke tests with fuzzing (100 iterations)
./scripts/smoke-test.sh all --fuzz 100
```

### Code Quality
```bash
# Format code with gofumpt
make fmt
# Or: mise run fmt
# Or: gofumpt -l -w .

# Run linter
make lint
# Or: mise run lint
# Or: golangci-lint run --timeout=5m

# Run static analysis
make check
# Or: mise run check
# Or: staticcheck ./...

# Run all quality checks (format, vet, lint, tidy, build, test)
make pre-commit
# Or: mise run pre-commit
```

### Demos
```bash
# Interactive REPL (manual testing, exploration)
mise run interactive
# Or: make interactive
# Commands: create, spawn <role>, msg <role> <message>, agents, help, quit

# Automated demo (quick overview of features)
mise run demo
# Or: make demo
```

### NATS (Optional)
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
# - Monitoring: http://localhost:8222
# - Metrics: http://localhost:7777/metrics
```

### Tool Management (mise)
```bash
# Install all development tools
mise install

# Verify installed tools and versions
mise list

# View available mise tasks
mise tasks
```

### Cleanup
```bash
# Remove build artifacts
make clean
# Removes: bin/ directory

# Clean up all (including worktrees, credentials, containers)
# NOTE: No automated command yet, manual cleanup required
```

## Workflow Commands

### Before Starting Work
```bash
mise install          # Ensure tools are installed
mise list            # Verify versions
```

### During Development
```bash
make build           # Check for compilation errors
make test            # Run tests frequently
```

### Before Committing
```bash
make pre-commit      # Run all checks (format, vet, lint, build, test)
# Or run individually:
make fmt
go vet ./...
make lint
make test
make build
```

### After Changes
```bash
git add .
git commit -m "your message"
# Pre-commit hooks will run automatically if installed
```

## Git Commands

### Setup
```bash
# Setup git worktrees for agents (if needed manually)
./scripts/setup-worktrees.sh
```

### Branches
```bash
# Current branch
git branch --show-current

# View all branches (including agent branches)
git branch -a

# Clean up stale worktrees
git worktree prune
```

## Docker Commands

### Containers
```bash
# List running containers (agents)
docker ps

# View all containers (including stopped)
docker ps -a

# View container logs
docker logs <container-id>

# Stop a container
docker stop <container-id>

# Remove a container
docker rm <container-id>

# Clean up all stopped containers
docker container prune
```

### Images
```bash
# List images
docker images

# Remove unused images
docker image prune
```

## Environment Setup

### Configure Environment Variables
```bash
# Copy example configuration
cp .envrc.example .envrc

# Edit with your settings
vim .envrc

# mise will automatically load it when you cd into the project
```

### Required Variables
```bash
export ANTHROPIC_API_KEY=sk-...   # Required for Claude Code agents
```

### Optional Variables
```bash
export OUROCODUS_ACP_BINARY=./bin/echo-agent  # Use custom ACP binary
export OUROCODUS_ACP_RUNTIME=container        # Run ACP in containers
export NATS_URL=nats://localhost:4222         # Enable NATS event logging
```

## CI/CD

### GitHub Actions Workflows
- **ci.yml**: Build, test, lint (runs on all PRs and main pushes)
- **smoke.yml**: Integration smoke tests (runs on all PRs and main pushes)

### Local CI Simulation
```bash
# Run same checks as CI
make pre-commit
mise run smoke
```

## Troubleshooting

### Tool not found
```bash
mise install         # Reinstall all tools
```

### Build failures
```bash
go mod tidy          # Clean dependencies
make clean           # Remove build artifacts
make build           # Rebuild
```

### Linting errors
```bash
golangci-lint run --fix  # Auto-fix some issues
make fmt                 # Format code
```

### Container issues
```bash
docker ps -a         # Check container status
docker logs <id>     # View container logs
docker restart <id>  # Restart container
```

### NATS connection issues
```bash
make nats-health     # Check NATS status
make nats-logs       # View NATS logs
make nats-stop       # Stop and restart
make nats-start
```
