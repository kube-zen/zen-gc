# CI/CD Documentation

This document describes the CI/CD pipeline and quality gates for zen-gc.

## Overview

zen-gc uses GitHub Actions for continuous integration, following Kubernetes and OSS best practices.

## CI Pipeline

### Workflows

The CI pipeline consists of the following jobs (defined in `.github/workflows/ci.yml`):

1. **Lint** - Code quality checks
2. **Test** - Unit tests with coverage
3. **Build** - Build verification
4. **Security** - Security scanning
5. **YAML Lint** - Manifest validation
6. **Verify** - Final verification

### Quality Gates

All PRs must pass:

- ✅ Code formatting (`gofmt`)
- ✅ Linting (`golangci-lint`)
- ✅ Static analysis (`go vet`)
- ✅ Unit tests (>80% coverage)
- ✅ Build verification
- ✅ Security scanning (`govulncheck`, `gosec`)
- ✅ YAML validation

## Local Development

### Pre-commit Checks

Install pre-commit hook:

```bash
cp .github/hooks/pre-commit .git/hooks/
chmod +x .git/hooks/pre-commit
```

The hook runs:
- Conflict marker detection
- Code formatting check
- `go vet` check

### Makefile Targets

```bash
make fmt              # Format code
make lint             # Run linter
make vet              # Run go vet
make test             # Run tests
make coverage         # Generate coverage report
make verify           # Run all checks
make ci-check         # Run full CI checks locally
make security-check   # Run security scans
```

### Manual CI Checks

Run all CI checks locally:

```bash
make ci-check
```

This runs:
1. Code formatting check
2. `go mod tidy` check
3. `go vet`
4. Build verification
5. Linter
6. Unit tests
7. Security checks

## GitHub Actions

### Workflow Triggers

- **Push** to `main` or `develop` branches
- **Pull Request** to `main` or `develop` branches

### Job Details

#### Lint Job

- Runs `golangci-lint` with `.golangci.yml` config
- Runs `go vet`
- Checks code formatting
- Verifies `go mod tidy`

#### Test Job

- Runs unit tests with race detection
- Generates coverage report
- Uploads coverage to Codecov

#### Build Job

- Builds the controller binary
- Verifies binary exists and is valid

#### Security Job

- Runs `govulncheck` for vulnerability scanning
- Runs `gosec` for security issues
- Uploads security reports

#### YAML Lint Job

- Validates YAML files in `deploy/` and `examples/`
- Uses `yamllint` with relaxed line length

## Coverage Requirements

- **Minimum**: 80% code coverage
- **Target**: >85% coverage
- **Critical paths**: >90% coverage

## Linting Configuration

Linting is configured in `.golangci.yml` with:

- **30+ linters** enabled
- **Kubernetes-style** configuration
- **OSS best practices**
- **Security-focused** checks

Key linters:
- `gofmt` - Code formatting
- `govet` - Static analysis
- `errcheck` - Error handling
- `staticcheck` - Advanced static analysis
- `gosec` - Security scanning
- `revive` - Style checking
- `gocritic` - Code quality

## Security Scanning

### Tools Used

1. **govulncheck** - Go vulnerability database
2. **gosec** - Security-focused linter
3. **Dependabot** - Dependency updates (GitHub)

### Running Security Checks

```bash
make security-check
```

## Best Practices

### Before Committing

1. Run `make fmt` to format code
2. Run `make lint` to check linting
3. Run `make test` to verify tests pass
4. Run `make verify` for full check

### Before Pushing

1. Run `make ci-check` to simulate CI
2. Ensure all tests pass
3. Check coverage hasn't decreased
4. Verify no security issues

### PR Requirements

- All CI checks must pass
- Coverage must be maintained (>80%)
- No security vulnerabilities
- Documentation updated
- Tests added for new features

## Troubleshooting

### Linter Errors

```bash
# Auto-fix some issues
golangci-lint run --fix

# See specific linter output
golangci-lint run -E errcheck
```

### Test Failures

```bash
# Run specific test
go test -v ./pkg/controller -run TestGCController

# Run with race detection
go test -race ./pkg/...
```

### Build Issues

```bash
# Clean and rebuild
make clean
make build

# Verify dependencies
go mod verify
```

## Continuous Deployment

CD is not yet configured. Future plans:

- Automated releases on tag
- Container image builds
- Helm chart publishing
- Documentation site updates

---

## References

- [GitHub Actions Workflow](.github/workflows/ci.yml)
- [Linting Configuration](.golangci.yml)
- [Makefile](Makefile)
- [Contributing Guide](../CONTRIBUTING.md)




