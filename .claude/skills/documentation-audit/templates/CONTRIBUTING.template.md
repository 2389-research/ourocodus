# Contributing Guide

Thank you for considering contributing to [Project Name]!

## Getting Started

### Development Setup

1. **Prerequisites**
   - Go [version] or later
   - [Other tools: make, docker, etc.]

2. **Clone and Setup**
   ```bash
   git clone https://github.com/[org]/[project].git
   cd [project]
   make setup
   ```

3. **Verify Setup**
   ```bash
   make test
   make lint
   ```

## Development Workflow

### 1. Create a Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/issue-description
```

### 2. Make Changes

- Write clear, focused commits
- Follow code style guidelines (see below)
- Add tests for new functionality
- Update documentation as needed

### 3. Test Your Changes

```bash
# Run all tests
make test

# Run specific test
go test ./path/to/package -v -run TestName

# Check coverage
make test-coverage
```

### 4. Lint and Format

```bash
# Format code
make fmt

# Run linters
make lint

# Fix auto-fixable issues
make lint-fix
```

### 5. Commit

Follow conventional commit format:

```
type(scope): brief description

Longer description if needed.

Closes #issue-number
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

### 6. Push and Create PR

```bash
git push origin feature/your-feature-name
```

Then create a Pull Request on GitHub.

## Code Style Guidelines

### Go Code

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Run `gofumpt` for formatting (stricter than `gofmt`)
- Use meaningful variable names
- Add comments for exported symbols
- Keep functions focused and small

### Documentation

- Document all exported types, functions, and constants
- Use complete sentences in doc comments
- Start doc comments with the name of the thing being documented
- Include examples for complex functionality
- Keep README and docs up to date

### Testing

- Write table-driven tests where appropriate
- Use subtests for related test cases
- Mock external dependencies
- Aim for high coverage on business logic
- Include edge cases and error conditions

## Pull Request Guidelines

### PR Checklist

Before submitting a PR, ensure:

- [ ] Tests pass locally (`make test`)
- [ ] Linters pass (`make lint`)
- [ ] Code is formatted (`make fmt`)
- [ ] New code has tests
- [ ] Documentation is updated
- [ ] Commit messages follow conventions
- [ ] PR description explains the change
- [ ] References related issues

### PR Review Process

1. CI checks must pass
2. At least one maintainer approval required
3. Address review feedback
4. Squash commits if requested
5. Maintainer will merge when ready

## Testing

### Running Tests

```bash
# All tests
make test

# With coverage
make test-coverage

# Specific package
go test ./pkg/package -v

# Integration tests
make test-integration
```

### Writing Tests

```go
func TestFeature(t *testing.T) {
    tests := []struct {
        name    string
        input   Input
        want    Output
        wantErr bool
    }{
        {
            name:  "normal case",
            input: Input{...},
            want:  Output{...},
        },
        // More test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Function(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Documentation

### Code Documentation

- All exported symbols must have doc comments
- Doc comments should be complete sentences
- Start with the name of the declared item
- Explain what, why, and any important details

```go
// ProcessRequest handles incoming HTTP requests and returns a response.
// It validates the input, processes the business logic, and formats the output.
// Returns an error if validation fails or processing encounters an issue.
func ProcessRequest(req Request) (Response, error) {
    // ...
}
```

### Project Documentation

Update these when making changes:

- **README.md**: User-facing changes, new features
- **docs/**: Detailed guides, architecture docs
- **CHANGELOG.md**: All notable changes (maintainers handle this)

## Issue Guidelines

### Reporting Bugs

Include:
- Clear description of the issue
- Steps to reproduce
- Expected vs actual behavior
- Environment details (OS, Go version, etc.)
- Relevant logs or error messages

### Feature Requests

Include:
- Use case and motivation
- Proposed solution or API
- Alternatives considered
- Willingness to contribute implementation

## Release Process

(Maintainers only)

1. Update CHANGELOG.md
2. Update version in relevant files
3. Create and push tag: `git tag v1.2.3 && git push origin v1.2.3`
4. CI will build and publish release

## Getting Help

- **Questions**: Use [Discussions](https://github.com/[org]/[project]/discussions)
- **Bugs**: Open an [Issue](https://github.com/[org]/[project]/issues)
- **Security**: See [SECURITY.md](SECURITY.md)

## Code of Conduct

Be respectful, inclusive, and professional. See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) for details.

## License

By contributing, you agree that your contributions will be licensed under the project's license.
