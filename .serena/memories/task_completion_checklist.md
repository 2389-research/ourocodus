# Task Completion Checklist

This checklist should be followed when completing any coding task in this project.

## Before Starting
- [ ] Read relevant documentation (architecture docs, session lifecycle, error handling)
- [ ] Understand the existing code structure and patterns
- [ ] Identify which subsystems are affected (relay, containersession, worktree, acp, nats, webapp)
- [ ] Check for related issues on GitHub

## During Development
- [ ] Follow Go code style (gofumpt formatting)
- [ ] Use structured error codes (see docs/development/ERROR_HANDLING.md)
- [ ] Keep cyclomatic complexity under 15
- [ ] Add tests for new functionality
- [ ] Update existing tests if behavior changes
- [ ] Add doc comments for exported symbols
- [ ] Consider concurrency and race conditions
- [ ] Validate input (especially paths and user input)
- [ ] Check for security issues (directory traversal, injection, etc.)

## Before Committing

### Code Quality
- [ ] Run `make fmt` to format code with gofumpt
- [ ] Run `go vet ./...` for basic static analysis
- [ ] Run `make lint` to check with golangci-lint
- [ ] Fix all linting errors and warnings
- [ ] Run `make check` for staticcheck analysis
- [ ] Run `go mod tidy` to clean dependencies

### Testing
- [ ] Run `make test` to ensure all unit tests pass
- [ ] Add new tests for new functionality
- [ ] Update tests if behavior changed
- [ ] Run `make test-e2e` if changing core functionality (requires ANTHROPIC_API_KEY)
- [ ] Consider running smoke tests: `mise run smoke`

### Build Verification
- [ ] Run `make build` to ensure compilation succeeds
- [ ] Test the binary if you modified cmd/ packages
- [ ] Run demos if you changed core features: `mise run demo` or `mise run interactive`

### Documentation
- [ ] Update relevant documentation in docs/
- [ ] Update README.md if adding new features or commands
- [ ] Update CHANGELOG.md if appropriate
- [ ] Add or update code comments for complex logic
- [ ] Check that doc comments are accurate

### Git
- [ ] Review your changes: `git diff`
- [ ] Stage files: `git add <files>`
- [ ] Write clear commit message describing what and why
- [ ] Use conventional commit format if possible (feat:, fix:, docs:, refactor:, test:)

### Full Pre-commit Check
- [ ] Run `make pre-commit` (runs: fmt, vet, lint, tidy, build, test)
- [ ] Or run `pre-commit run --all-files` if hooks are installed

## After Committing

### Pull Requests
- [ ] Push branch: `git push origin <branch-name>`
- [ ] Create PR on GitHub with clear description
- [ ] Link related issues in PR description
- [ ] Check that CI passes (ci.yml and smoke.yml workflows)
- [ ] Address any CI failures
- [ ] Respond to code review feedback
- [ ] Ensure all conversations are resolved

### Merging
- [ ] Ensure all tests pass
- [ ] Ensure CI is green
- [ ] Get approval from maintainers
- [ ] Squash and merge (or follow project merge policy)
- [ ] Delete feature branch after merge

## Common Pitfalls to Avoid
- [ ] Don't ignore errors (errcheck linter will catch this)
- [ ] Don't create functions with cyclomatic complexity > 15
- [ ] Don't skip formatting (always run `make fmt`)
- [ ] Don't commit without running tests
- [ ] Don't leave debug print statements in production code
- [ ] Don't hard-code paths or configuration
- [ ] Don't forget to handle context cancellation
- [ ] Don't create race conditions (use sync primitives properly)
- [ ] Don't expose internal errors to users (use structured error codes)
- [ ] Don't commit binary files or build artifacts

## Quick Command Reference
```bash
# Full pre-commit workflow
make pre-commit

# Or run individually:
make fmt                    # Format code
go vet ./...                # Static analysis
make lint                   # Comprehensive linting
go mod tidy                 # Clean dependencies
make build                  # Build binaries
make test                   # Run tests

# Optional but recommended:
make test-e2e               # E2E tests (requires API key)
mise run smoke              # Smoke tests
mise run demo               # Test demo functionality
```

## Emergency Fixes
If you need to quickly fix a critical bug:
1. Create hotfix branch from main
2. Make minimal changes to fix the bug
3. Add test that would have caught the bug
4. Run full pre-commit checks
5. Create PR with "hotfix:" prefix
6. Get fast-track review
7. Merge and tag release if needed
