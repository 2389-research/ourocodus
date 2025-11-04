# Task Completion Checklist

When you complete a coding task in Ourocodus, follow this checklist to ensure quality and consistency.

## Before Committing Changes

### 1. Format Code
```bash
make fmt
# or
mise run fmt
```
This runs `gofumpt -l -w .` to format all Go files with stricter rules than gofmt.

### 2. Run Static Analysis
```bash
go vet ./...
```
Catches suspicious constructs and common mistakes.

### 3. Run Linter
```bash
make lint
# or
mise run lint
```
This runs `golangci-lint run --timeout=5m` to check for code quality issues.

**Note**: Some issues can be auto-fixed:
```bash
golangci-lint run --fix
```

### 4. Run Static Checker
```bash
make check
# or
mise run check
```
This runs `staticcheck ./...` for advanced static analysis.

### 5. Clean Dependencies
```bash
go mod tidy
```
Ensures `go.mod` and `go.sum` are clean and up-to-date.

### 6. Build the Project
```bash
make build
# or
mise run build
```
Verifies that all binaries compile successfully.

### 7. Run Tests
```bash
make test
# or
mise run test
```
Runs the full test suite with `go test ./...`.

**For integration tests** (if applicable):
```bash
go test -tags=integration ./pkg/agent/packnplay/... -v
```

**For end-to-end tests** (if applicable and you have ANTHROPIC_API_KEY):
```bash
make test-e2e
```

### 8. Run All Pre-commit Checks (Recommended)
```bash
make pre-commit
# or
mise run pre-commit
```
This runs all the above checks in sequence: fmt, vet, lint, tidy, build, test.

## Quick Pre-commit Command

Instead of running steps 1-7 individually, you can run:
```bash
mise run pre-commit
```
This is the **recommended** approach before committing.

## Integration Test Cleanup

If you ran integration tests and they failed or were interrupted:

```bash
# Check for orphaned containers
docker ps -a --filter "label=managed-by=packnplay"

# Clean up if needed
docker ps -a --filter "label=managed-by=packnplay" -q | xargs docker rm -f

# Remove associated worktrees (if needed)
rm -rf ~/.local/share/packnplay/worktrees
```

## Git Workflow

After all checks pass:

1. **Stage changes**:
   ```bash
   git add .
   ```

2. **Commit with descriptive message**:
   ```bash
   git commit -m "feat: add feature description"
   ```
   Follow conventional commit format when possible.

3. **Push to remote**:
   ```bash
   git push
   ```

## CI/CD Checks

The following checks run automatically on GitHub Actions for all PRs and pushes to main:

**ci.yml workflow:**
- Build all binaries
- Run unit tests
- golangci-lint verification
- gofmt formatting check
- shellcheck on scripts
- Binary smoke test

**smoke.yml workflow:**
- Session management smoke tests
- WebSocket relay integration tests
- Error handling tests

Make sure your changes pass locally before pushing to avoid CI failures.

## Optional: Pre-commit Hooks

If you have pre-commit hooks installed:
```bash
pre-commit install
```

The hooks will automatically run the pre-commit checks before each commit.

## Summary

The simplest approach:
```bash
# Run all checks at once
make pre-commit

# If all pass, commit your changes
git add .
git commit -m "your message"
git push
```
