# Contributing to zen-gc

Thank you for your interest in contributing to zen-gc! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Coding Standards](#coding-standards)
- [Testing](#testing)
- [Documentation](#documentation)
- [Submitting Changes](#submitting-changes)
- [Review Process](#review-process)

---

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

---

## Getting Started

### Prerequisites

- Go 1.23 or later
- Kubernetes cluster (kind/minikube) for E2E tests
- kubectl configured
- golangci-lint (for linting)

### Setting Up Development Environment

1. **Fork and clone the repository:**
   ```bash
   git clone https://github.com/kube-zen/zen-gc.git
   cd zen-gc
   ```

2. **Install dependencies:**
   ```bash
   make deps
   ```

3. **Run tests to verify setup:**
   ```bash
   make test
   ```

4. **Install development tools:**
   ```bash
   # Install golangci-lint
   curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin latest
   
   # Install pre-commit hooks (optional)
   cp .github/hooks/pre-commit .git/hooks/
   chmod +x .git/hooks/pre-commit
   ```

---

## Development Workflow

### 1. Create a Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/your-bug-fix
```

**Branch naming conventions:**
- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation changes
- `refactor/` - Code refactoring
- `test/` - Test improvements

### 2. Make Changes

- Write clean, well-documented code
- Follow Kubernetes coding standards
- Add tests for new functionality
- Update documentation as needed

### 3. Run Quality Checks

Before committing, run:

```bash
# Format code
make fmt

# Run linter
make lint

# Run tests
make test

# Check coverage
make coverage

# Run all checks
make verify
```

### 4. Commit Changes

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `style`: Formatting
- `refactor`: Code refactoring
- `test`: Tests
- `chore`: Maintenance

**Example:**
```
feat(controller): add exponential backoff for deletions

Implement exponential backoff retry logic for resource deletions
to handle transient API server errors gracefully.

Fixes #123
```

### 5. Push and Create Pull Request

```bash
git push origin feature/your-feature-name
```

Then create a Pull Request on GitHub.

---

## Coding Standards

### Go Style Guide

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Follow [Kubernetes Code Style](https://github.com/kubernetes/community/blob/master/contributors/guide/coding-conventions.md)
- Use `gofmt` for formatting
- Use `golint` and `golangci-lint` for linting

### Code Organization

```
zen-gc/
├── cmd/              # Application entrypoints
├── pkg/              # Library code
│   ├── api/         # API types
│   ├── controller/  # Controller logic
│   └── validation/  # Validation logic
├── deploy/          # Deployment manifests
├── docs/            # Documentation
├── examples/        # Example policies
└── test/            # Tests
```

### Naming Conventions

- **Packages**: lowercase, single word
- **Exported functions**: PascalCase
- **Unexported functions**: camelCase
- **Constants**: PascalCase or UPPER_CASE
- **Variables**: camelCase

### Error Handling

- Always check errors
- Use `fmt.Errorf` with `%w` for error wrapping
- Provide context in error messages
- Return errors, don't log and ignore

**Good:**
```go
if err != nil {
    return fmt.Errorf("failed to delete resource: %w", err)
}
```

**Bad:**
```go
if err != nil {
    klog.Errorf("Error: %v", err)
    // continue anyway
}
```

### Documentation

- Add godoc comments for all exported functions
- Document complex logic
- Include examples in godoc
- Update README/docs for user-facing changes

**Example:**
```go
// DeleteResource deletes a Kubernetes resource based on the GC policy.
// It respects rate limiting and handles dry-run mode.
//
// Parameters:
//   - resource: The resource to delete
//   - policy: The GC policy that triggered the deletion
//
// Returns:
//   - error: Any error encountered during deletion
//
// Example:
//   err := controller.DeleteResource(resource, policy)
//   if err != nil {
//       return fmt.Errorf("deletion failed: %w", err)
//   }
func DeleteResource(resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy) error {
    // ...
}
```

---

## Testing

### Unit Tests

- Write tests for all new functionality
- Aim for >80% code coverage
- Use table-driven tests where appropriate
- Test both success and error cases

**Example:**
```go
func TestCalculateTTL(t *testing.T) {
    tests := []struct {
        name     string
        input    *TTLSpec
        expected int64
        wantErr  bool
    }{
        {
            name: "fixed TTL",
            input: &TTLSpec{SecondsAfterCreation: int64Ptr(3600)},
            expected: 3600,
            wantErr: false,
        },
        // ... more test cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

### Integration Tests

- Test with fake Kubernetes client
- Test CRUD operations
- Test error scenarios

### E2E Tests

- Test with kind/minikube
- Test full GC flow
- Test error scenarios

**Run tests:**
```bash
make test              # Unit tests
make test-integration  # Integration tests
make test-e2e          # E2E tests (requires cluster)
```

---

## Documentation

### Code Documentation

- Add godoc comments to all exported functions, types, and packages
- Document complex algorithms
- Include usage examples

### User Documentation

- Update `docs/USER_GUIDE.md` for user-facing changes
- Update `docs/API_REFERENCE.md` for API changes
- Update `docs/OPERATOR_GUIDE.md` for operational changes

### KEP Documentation

- Update `docs/KEP_GENERIC_GARBAGE_COLLECTION.md` for design changes
- Keep examples up to date in `examples/`

---

## Submitting Changes

### Pull Request Checklist

Before submitting a PR, ensure:

- [ ] Code follows style guidelines
- [ ] All tests pass (`make test`)
- [ ] Code is formatted (`make fmt`)
- [ ] Linter passes (`make lint`)
- [ ] Coverage is maintained (>80%)
- [ ] Documentation is updated
- [ ] Commit messages follow conventions
- [ ] PR description explains changes
- [ ] Related issues are referenced

### PR Description Template

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
How was this tested?

## Checklist
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] Code follows style guidelines
- [ ] Self-review completed

## Related Issues
Fixes #123
```

---

## Review Process

1. **Automated Checks**: CI runs automatically on PR
2. **Code Review**: At least one maintainer must approve
3. **Testing**: All tests must pass
4. **Documentation**: Documentation must be updated
5. **Merge**: Squash and merge (preferred) or merge commit

### Review Criteria

- Code quality and style
- Test coverage
- Documentation completeness
- Backward compatibility
- Performance implications
- Security considerations

---

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.

---

## Questions?

- Open an issue for questions
- Check existing documentation
- Review existing code for examples

Thank you for contributing! 🎉

