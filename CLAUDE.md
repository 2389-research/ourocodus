# Development Tools Guide for AI Agents

This document outlines how to use the development tools configured in this repository. This is intended for AI coding agents (Claude Code, Cursor, GitHub Copilot, etc.) working on this codebase.

## Tool Management

This project uses [mise](https://mise.jdx.dev/) to manage development tools and ensure consistent versions across all developers and agents.

### Installing Tools

All required development tools can be installed with a single command:

```bash
mise install
```

This installs:

- **Go 1.23.0** - Programming language runtime
- **golangci-lint** - Comprehensive linter
- **staticcheck** - Advanced static analysis tool
- **gofumpt** - Stricter Go formatter

### Checking Tool Versions

To verify which tools are installed and their versions:

```bash
mise list
```

## Code Quality Tools

### 1. Formatting with gofumpt

**Purpose**: Format Go code with stricter rules than standard `gofmt`.

**When to use**: Before committing any Go code changes.

**Usage**:

```bash
# Via mise
mise run fmt

# Via Makefile
make fmt

# Direct command
gofumpt -l -w .
```

**What it does**:

- Formats all Go files in the current directory and subdirectories
- Enforces stricter formatting rules than `gofmt`
- Modifies files in place (`-w` flag)

**Expected behavior**: No output if all files are properly formatted. Lists files that were formatted if changes were made.

### 2. Linting with golangci-lint

**Purpose**: Run comprehensive linting checks to catch bugs, style issues, and code smells.

**When to use**: After making code changes, before committing.

**Usage**:

```bash
# Via mise
mise run lint

# Via Makefile
make lint

# Direct command
golangci-lint run --timeout=5m
```

**What it checks**:

- Code formatting (gofmt)
- Static analysis (govet)
- Error checking (errcheck)
- Unused code (unused)
- Code complexity (gocyclo)
- Security issues (gosec)
- And many more (see `.golangci.yml`)

**Configuration**: See `.golangci.yml` for enabled linters and settings.

**Expected behavior**:

- No output if all checks pass
- Detailed error messages with file locations if issues are found
- Exit code 0 on success, non-zero on failure

### 3. Static Analysis with staticcheck

**Purpose**: Perform advanced static analysis to find bugs and performance issues.

**When to use**: After making significant code changes.

**Usage**:

```bash
# Via mise
mise run check

# Via Makefile
make check

# Direct command
staticcheck ./...
```

**What it finds**:

- Unused code
- Incorrect API usage
- Performance issues
- Potential bugs
- Code simplification opportunities

**Expected behavior**: No output if all checks pass. Detailed warnings/errors if issues are found.

### 4. Built-in Go Tools

#### go vet

**Purpose**: Examines Go source code and reports suspicious constructs.

**Usage**:

```bash
go vet ./...
```

**When to use**: Included in pre-commit checks, can be run manually for quick validation.

#### go test

**Purpose**: Run all tests in the project.

**Usage**:

```bash
# Via Makefile
make test

# Direct command
go test ./...
```

**When to use**: After making code changes, before committing.

## Recommended Workflow for AI Agents

When making changes to this codebase, follow this workflow:

### 1. Before Starting Work

```bash
# Ensure tools are installed
mise install

# Verify environment
mise list
```

### 2. During Development

```bash
# Build to check for compilation errors
make build

# Run tests frequently
make test
```

### 2.5. Integration Tests

Some packages have integration tests that require external dependencies (e.g., Docker). These tests are gated behind build tags and do not run by default.

#### Packnplay Integration Tests

The `pkg/agent/packnplay` package includes integration tests that spawn real Docker containers to verify the full agent lifecycle.

**Requirements:**
- Docker daemon running and accessible
- Git repository (tests run in current repo)
- Network access to pull container images (busybox, etc.)

**Running integration tests:**

```bash
# Run all integration tests
go test -tags=integration ./pkg/agent/packnplay/... -v

# Run specific integration test
go test -tags=integration ./pkg/agent/packnplay/... -run TestIntegration_SpawnAndStop -v

# Run with timeout (recommended for CI)
go test -tags=integration ./pkg/agent/packnplay/... -v -timeout=5m
```

**What integration tests verify:**
- Container spawning and lifecycle (Spawn/Stop)
- Live I/O streaming (stdin/stdout/stderr)
- Attach to existing containers
- Agent discovery via Docker labels
- Cleanup and error scenarios
- Wait for container exit

**Cleanup:**

Integration tests automatically clean up containers on success. If tests fail or are interrupted, orphaned containers may remain. Clean them up with:

```bash
# List Packnplay-managed containers
docker ps -a --filter "label=managed-by=packnplay"

# Remove all Packnplay containers
docker ps -a --filter "label=managed-by=packnplay" -q | xargs docker rm -f

# Remove associated worktrees (if needed)
rm -rf ~/.local/share/packnplay/worktrees
```

**CI/CD Considerations:**

Integration tests are not run in CI by default due to Docker requirements. To enable them:
1. Ensure runner has Docker access
2. Add `-tags=integration` to test command
3. Consider using Docker-in-Docker or privileged runner

### 3. Before Committing

Run all quality checks:

```bash
# Option 1: Run all checks at once
mise run pre-commit

# Option 2: Run all checks via Makefile
make pre-commit

# Option 3: Run checks individually
make fmt      # Format code
go vet ./...  # Basic static analysis
make lint     # Comprehensive linting
make test     # Run tests
make build    # Verify build
```

The `pre-commit` task runs:

1. `gofumpt -l -w .` - Format all Go files
2. `go vet ./...` - Basic static analysis
3. `golangci-lint run --timeout=5m` - Comprehensive linting
4. `go mod tidy` - Clean up dependencies
5. `make build` - Verify project builds

### 4. Automated Pre-commit Hooks

If pre-commit hooks are installed, they will automatically run before each commit:

```bash
pre-commit install
```

The hooks will automatically run formatting, linting, and building checks.

## Common Issues and Solutions

### Issue: Tool not found

**Error**: `command -v gofumpt: command not found` or similar

**Solution**:

```bash
mise install
```

If mise is not installed, see the installation instructions in `CONTRIBUTING.md`.

### Issue: Linting errors

**Error**: `golangci-lint` reports errors

**Solution**:

1. Review the error messages carefully
2. Fix the reported issues
3. Some issues can be auto-fixed with `golangci-lint run --fix`
4. Re-run `make lint` to verify fixes

### Issue: Formatting differences

**Error**: Files are not formatted correctly

**Solution**:

```bash
make fmt
```

This will automatically format all Go files according to gofumpt rules.

### Issue: Build failures

**Error**: `make build` fails

**Solution**:

1. Check the error message for specific compilation errors
2. Fix the code issues
3. Run `make build` again
4. If dependency issues, try `go mod tidy`

## Tool Configuration Files

- **`.mise.toml`** - Tool versions and mise tasks
- **`.golangci.yml`** - golangci-lint configuration (enabled linters, settings)
- **`.pre-commit-config.yaml`** - Pre-commit hooks configuration
- **`Makefile`** - Build and quality check targets

## Integration with CI/CD

All quality checks run automatically in GitHub Actions on:

- All pull requests
- Pushes to main branch

The CI pipeline runs:

- Build verification
- Test suite
- Linting with golangci-lint
- Format checking with gofmt (note: local dev uses gofumpt which is stricter)

## Quick Reference

| Task | Command | Description |
|------|---------|-------------|
| Install tools | `mise install` | Install all development tools |
| Format code | `make fmt` or `mise run fmt` | Format with gofumpt |
| Lint code | `make lint` or `mise run lint` | Run golangci-lint |
| Static analysis | `make check` or `mise run check` | Run staticcheck |
| Run tests | `make test` | Run test suite |
| Build project | `make build` | Build all binaries |
| All checks | `make pre-commit` or `mise run pre-commit` | Run all quality checks |
| Clean build | `make clean` | Remove build artifacts |

## For More Information

- **General contribution guidelines**: See `CONTRIBUTING.md`
- **Project overview**: See `README.md`
- **mise documentation**: <https://mise.jdx.dev/>
- **golangci-lint documentation**: <https://golangci-lint.run/>
