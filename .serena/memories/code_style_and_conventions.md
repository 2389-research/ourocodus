# Code Style and Conventions

## Go Code Style

### Formatting
- **Primary formatter**: `gofumpt` (stricter than gofmt)
  - Run via: `make fmt` or `mise run fmt`
  - Auto-formats all Go files
  - Required before committing

### Linting
- **Primary linter**: `golangci-lint`
  - Configuration: `.golangci.yml`
  - Run via: `make lint` or `mise run lint`
  - Timeout: 5 minutes
  - Enabled linters: gofmt, govet, errcheck, staticcheck, unused, gosimple, ineffassign, typecheck, revive, gocyclo, misspell, unparam, unconvert, gosec

### Static Analysis
- **Tool**: `staticcheck`
  - Run via: `make check` or `mise run check`
  - Advanced static analysis for bug detection

### Complexity
- **Max cyclomatic complexity**: 15 (enforced by gocyclo)
- Complex functions should be broken down
- Test files exempt from complexity checks

### Error Handling
- All errors must be checked (errcheck linter)
- Use structured error codes (see docs/development/ERROR_HANDLING.md)
- Test files have relaxed error checking rules

### Naming Conventions
- **Package names**: Lowercase, single word (e.g., relay, acp, worktree)
- **Interfaces**: Descriptive names ending in -er when appropriate (e.g., Manager, Launcher)
- **Structs**: PascalCase for exported, camelCase for unexported
- **Methods**: Follow Go conventions (e.g., New*, Get*, Create*, List*)
- **Constants**: PascalCase or SCREAMING_SNAKE_CASE for error codes

### Testing
- Test files: `*_test.go`
- Test functions: `func TestXxx(t *testing.T)`
- Use testify for assertions: `github.com/stretchr/testify`
- Table-driven tests preferred for multiple cases
- Test coverage encouraged but not enforced

### Documentation
- Exported symbols must have doc comments
- Doc comments start with the symbol name
- Example: `// Manager coordinates session lifecycle.`

## TypeScript/JavaScript Style (PWA)

### Files
- TypeScript source: `internal/webapp/src/*.ts`
- Configuration: `internal/webapp/src/tsconfig.json`
- Testing: Vitest (`internal/webapp/src/vitest.config.js`)

### Build
- Bundler: esbuild
- Minifier: minify
- Output: `internal/webapp/web/` (with content-hashed filenames)

## Project-Specific Patterns

### Session IDs
- UUIDs generated via `github.com/google/uuid`
- Format: `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`
- Used for UserSession and AgentSession tracking

### Agent Identifiers
- User-chosen strings (e.g., "coder-1", "analyzer", "db-specialist")
- No predefined types or limits
- Validated for path safety (no directory traversal)

### Branch Naming
- Pattern: `agent-{agentID}-{timestamp}`
- Example: `agent-coder-1-20250117-153045`
- Ensures unique branches per agent

### Workspace Paths
- Base directory: `./workspaces` (configurable)
- Per-agent: `./workspaces/agent-{agentID}/`
- Validated for security (no ../ traversal)

### Error Codes
- Structured error codes (e.g., SESSION_NOT_FOUND, AGENT_NOT_FOUND)
- Includes recoverability flag (recoverable: true/false)
- See docs/development/ERROR_HANDLING.md

## Pre-commit Workflow

Before committing, run:
```bash
make pre-commit
```

This runs:
1. `gofumpt -l -w .` - Format code
2. `go vet ./...` - Basic static analysis
3. `golangci-lint run --timeout=5m` - Comprehensive linting
4. `go mod tidy` - Clean dependencies
5. `make build` - Verify build succeeds

Or use pre-commit hooks:
```bash
pre-commit install
pre-commit run --all-files
```

## Darwin-Specific Notes
- System: macOS (Darwin 24.6.0 in current env)
- Docker: Colima or Docker Desktop
- Shell: Bash or Zsh
- Tool management: mise (instead of manual installs)
