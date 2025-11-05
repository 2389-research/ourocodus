# Contributing to Ourocodus

Thank you for your interest in contributing to Ourocodus! This guide will help you set up your development environment and understand our development workflow.

## Development Environment Setup

### Using mise (Recommended)

We use [mise](https://mise.jdx.dev/) to manage development tools and ensure everyone has the same versions. This eliminates the "works on my machine" problem.

#### Install mise

**macOS:**

```bash
brew install mise
```

**Linux:**

```bash
curl https://mise.run | sh
```

**Other platforms:**
See [mise installation docs](https://mise.jdx.dev/getting-started.html)

#### Activate mise

Add to your shell profile (`~/.bashrc`, `~/.zshrc`, etc.):

```bash
eval "$(mise activate bash)"  # or zsh, fish, etc.
```

Then reload your shell or run:

```bash
source ~/.bashrc  # or your shell config file
```

#### Install Tools

Once mise is installed and activated, navigate to the project directory and run:

```bash
cd ourocodus
mise install
```

This will automatically install:

- **Go 1.23.0** - Programming language
- **golangci-lint** - Comprehensive linter
- **staticcheck** - Advanced static analysis
- **gofumpt** - Stricter Go formatter

#### Configure Environment (Optional)

For custom environment variables:

```bash
# Copy the example file
cp .envrc.example .envrc

# Edit with your settings
vim .envrc

# mise will automatically load it
```

#### Verify Installation

```bash
mise list
```

You should see all tools listed with their versions.

### Manual Setup (Without mise)

If you prefer not to use mise, install these tools manually:

```bash
# Install Go 1.23
# See https://go.dev/doc/install

# Install golangci-lint
brew install golangci-lint  # macOS
# or: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Install staticcheck
go install honnef.co/go/tools/cmd/staticcheck@latest

# Install gofumpt
go install mvdan.cc/gofumpt@latest
```

## Local Services with Docker

Ourocodus uses Docker Compose to run local development services including NATS for message streaming.

### Prerequisites

**Docker**: You need Docker installed and running. On macOS, we recommend [Colima](https://github.com/abiosoft/colima):

```bash
# Install Colima (macOS)
brew install colima

# Start Colima
colima start
```

For other platforms, install [Docker Desktop](https://www.docker.com/products/docker-desktop/) or your preferred Docker runtime.

### Starting Services

Start all services (currently: NATS with JetStream):

```bash
docker-compose up -d
```

This will:
1. Start NATS server with JetStream enabled
2. Create required JetStream streams (SESSION_EVENTS, WORK_RESULTS)
3. Expose NATS on ports:
   - 4222: Client connections
   - 8222: HTTP monitoring

### Verify Services

Check that services are running:

```bash
# Check container status
docker-compose ps

# View NATS logs
docker-compose logs nats

# View init script output
docker-compose logs nats-init
```

Access NATS monitoring endpoints:

```bash
# Health check
curl http://localhost:8222/healthz

# Server stats (includes JetStream info)
curl http://localhost:8222/varz | jq

# JetStream-specific stats
curl http://localhost:8222/jsz | jq
```

### NATS CLI Commands

If you have the [NATS CLI](https://github.com/nats-io/natscli) installed, you can interact with NATS directly:

```bash
# List JetStream streams
nats stream list

# View stream details
nats stream info SESSION_EVENTS
nats stream info WORK_RESULTS

# Publish a test message
nats pub "sessions.test-123.events" "test message"

# Subscribe to messages
nats sub "sessions.*.events"
```

### Stopping Services

Stop all services:

```bash
docker-compose down
```

To also remove volumes (clears all JetStream data):

```bash
docker-compose down -v
```

## Code Quality Tools

### Linting

Run the linter before committing:

```bash
# Using make (recommended)
make lint

# Using mise
mise run lint

# Or directly
golangci-lint run --timeout=5m
```

The linter configuration is in `.golangci.yml` and includes:

- gofmt, govet, errcheck, staticcheck
- unused, gosimple, ineffassign, typecheck
- revive, gocyclo, misspell, unparam
- unconvert, gosec

### Formatting

We use `gofumpt` for stricter formatting than `gofmt`:

```bash
# Using make (recommended)
make fmt

# Using mise
mise run fmt

# Or directly
gofumpt -l -w .
```

### Static Analysis

Run staticcheck for advanced static analysis:

```bash
# Using make (recommended)
make check

# Using mise
mise run check

# Or directly
staticcheck ./...
```

### Run All Checks

To run all quality checks at once:

```bash
# Using make (recommended)
make pre-commit

# Using mise
mise run pre-commit
```

### Pre-commit Hooks

We use [pre-commit](https://pre-commit.com/) to run checks automatically before each commit.

#### Install pre-commit

```bash
pip install pre-commit  # or: brew install pre-commit
```

#### Install hooks

```bash
pre-commit install
```

#### What the hooks do

The pre-commit hooks will automatically:

- Format code with gofumpt
- Run go vet for basic static analysis
- Organize imports with go-imports
- Clean dependencies with go mod tidy
- Run golangci-lint
- Build the project to catch compilation errors
- Check for trailing whitespace and other common issues

#### Run manually

```bash
# Run on all files
pre-commit run --all-files

# Run on staged files
pre-commit run
```

## Building and Testing

### Build

```bash
make build
```

This produces binaries in `bin/`:

- `bin/relay` - WebSocket relay server
- `bin/cli` - Command-line interface
- `bin/echo-agent` - Echo test agent

### Test

```bash
make test
```

Runs the test suite with `go test ./...`

### Run

```bash
make run
```

Starts the relay server.

### Clean

```bash
make clean
```

Removes build artifacts.

## Development Workflow

1. **Pick an issue** from [GitHub Issues](https://github.com/2389-research/ourocodus/issues)
2. **Create a branch** from `main`
3. **Make your changes** with frequent commits
4. **Run quality checks**:

   ```bash
   mise run lint
   mise run fmt
   make test
   ```

5. **Verify documentation** (see [DOCUMENTATION.md](DOCUMENTATION.md) for full guide):

   - [ ] **Public API Changes**: Exported functions/types have godoc comments
   - [ ] **User-Facing Changes**: README.md features/usage sections updated
   - [ ] **Security Changes**: Threat model documented in docs/SECURITY.md
   - [ ] **Examples Updated**: Behavior changes reflected in examples/
   - [ ] **Architecture Changes**: docs/ARCHITECTURE.md updated if structure changed
   - [ ] **Configuration Changes**: README.md environment variables updated
   - [ ] **Breaking Changes**: Migration guide added to PR description

6. **Push and create a PR**
7. **Address review feedback**

## Project Structure

```text
ourocodus/
├── cmd/                  # Binary entry points
│   ├── relay/           # WebSocket relay server
│   ├── cli/             # Command-line interface
│   └── echo-agent/      # Echo test agent
├── pkg/                  # Shared packages
│   └── acp/             # Agent Client Protocol
├── web/                  # PWA frontend (future)
├── scripts/              # Build and setup scripts
├── docs/                 # Documentation
├── .mise.toml           # Development tool versions
├── .golangci.yml        # Linter configuration
└── .pre-commit-config.yaml  # Pre-commit hooks
```

## Testing Guidelines

- Write tests for new functionality
- Maintain existing test coverage
- Tests should be fast and focused
- Use table-driven tests where appropriate

## Code Style

- Follow standard Go conventions
- Use `gofumpt` for formatting (stricter than `gofmt`)
- Keep functions small and focused
- Document exported functions and types
- Handle errors explicitly

## Getting Help

- Check existing [documentation](docs/)
- Review [open issues](https://github.com/2389-research/ourocodus/issues)
- Ask questions in issue comments
- Join project discussions

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
