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
- **Go 1.23** - Programming language
- **golangci-lint** - Comprehensive linter
- **staticcheck** - Advanced static analysis
- **gofumpt** - Stricter Go formatter

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

## Code Quality Tools

### Linting

Run the linter before committing:

```bash
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
# Using mise
mise run fmt

# Or directly
gofumpt -l -w .
```

### Static Analysis

Run staticcheck for advanced static analysis:

```bash
# Using mise
mise run check

# Or directly
staticcheck ./...
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
5. **Push and create a PR**
6. **Address review feedback**

## Project Structure

```
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
