# Code Style and Conventions

This document describes the code style and conventions used in the Ourocodus project.

## Go Version

The project uses **Go 1.24.0**. Always ensure compatibility with this version.

## Formatting

- **Tool**: `gofumpt` (stricter than standard `gofmt`)
- **Command**: `make fmt` or `gofumpt -l -w .`
- **Rule**: All Go code must be formatted with gofumpt before committing
- **Note**: CI checks use gofmt, but local development uses gofumpt which is stricter

## Linting

The project uses `golangci-lint` with configuration in `.golangci.yml`.

**Enabled linters include:**
- gofmt - Code formatting
- govet - Static analysis
- errcheck - Error checking
- unused - Unused code detection
- gocyclo - Code complexity
- gosec - Security issues
- And many more (see `.golangci.yml`)

**Command**: `make lint` or `golangci-lint run --timeout=5m`

## Static Analysis

- **Tool**: `staticcheck`
- **Command**: `make check` or `staticcheck ./...`
- **Purpose**: Find bugs, performance issues, and code simplification opportunities

## Naming Conventions

Follow standard Go naming conventions:
- **Packages**: Short, lowercase, single-word names (e.g., `relay`, `session`, `acp`)
- **Files**: Lowercase with underscores (e.g., `session_manager.go`, `client_factory.go`)
- **Types**: PascalCase (e.g., `SessionManager`, `AgentLauncher`)
- **Functions/Methods**: PascalCase for exported, camelCase for unexported
- **Variables**: camelCase (e.g., `sessionID`, `agentRole`)
- **Constants**: PascalCase or SCREAMING_SNAKE_CASE depending on context
- **Interfaces**: Typically end with `-er` suffix (e.g., `Launcher`, `Manager`, `Factory`)

## Documentation

- **Package docs**: Every package should have a `doc.go` file or package comment
- **Exported symbols**: All exported functions, types, and methods must have doc comments
- **Doc format**: Follow standard Go documentation conventions (complete sentences starting with the symbol name)

Example:
```go
// SessionManager manages the lifecycle of agent sessions.
// It coordinates session creation, agent spawning, and cleanup.
type SessionManager struct {
    // ...
}
```

## Error Handling

- Always check and handle errors
- Use structured error codes (see `pkg/relay/session/errors.go`)
- Provide context with error wrapping: `fmt.Errorf("context: %w", err)`
- Return errors rather than panicking in production code
- Define custom error types when needed for specific error handling

## Testing

- **Framework**: `github.com/stretchr/testify`
- **Test files**: Name with `_test.go` suffix
- **Integration tests**: Use build tags: `//go:build integration`
- **Test naming**: `Test<FunctionName>_<Scenario>` (e.g., `TestCreateSession_Success`)
- **Table-driven tests**: Use for multiple scenarios
- **Mocks**: Generate mocks for interfaces (e.g., `mock_launcher.go`)

**Example integration test:**
```go
//go:build integration

package packnplay_test

func TestIntegration_SpawnAndStop(t *testing.T) {
    // ...
}
```

## Project-Specific Patterns

### Agent Launcher Abstraction

The project uses an `AgentLauncher` interface to abstract agent lifecycle management:
```go
type AgentLauncher interface {
    Spawn(ctx context.Context, role, workspace string) (AgentHandle, error)
}
```

### Session Management

Sessions are identified by UUIDs and track multiple agents by role:
- Session lifecycle: Create → Spawn Agents → Communicate → Cleanup
- State transitions: PENDING → SPAWNING → ACTIVE → STOPPED → ERROR

### Docker Labels

Containers managed by Packnplay use labels:
- `managed-by=packnplay` - Identifies Packnplay-managed containers
- Session and role information stored in labels for discovery

### NATS Topic Naming

- Session events: `sessions.<session-id>.events`
- Work distribution: `sessions.<session-id>.work.<role>`
- Work results: `sessions.<session-id>.results.<role>`
- Agent heartbeats: `agents.<session-id>.<role>.heartbeat`

## Dependencies

- **Pinning**: Core dependencies like Packnplay are explicitly pinned to stable releases
- **Updating**: Use `go get package@version && go mod tidy`
- **Licenses**: Document third-party licenses in `NOTICE` file

## Git Commit Messages

Follow conventional commit format when possible:
- `feat:` - New features
- `fix:` - Bug fixes
- `docs:` - Documentation changes
- `test:` - Test additions or modifications
- `refactor:` - Code refactoring
- `chore:` - Build process or auxiliary tool changes

## Build Tags

Use build tags for conditional compilation:
- `//go:build integration` - Integration tests requiring Docker
- Tag-specific tests don't run by default with `go test ./...`
