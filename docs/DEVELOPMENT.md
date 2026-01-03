# Development Guide

This guide covers development setup, workflows, and best practices for {{ .projectName }}.

## Prerequisites

- Go 1.24 (see [Go Toolchain](#go-toolchain) section)
- kubectl configured to access a Kubernetes cluster
- Docker (for building images)
- Make

## Installation

```bash
git clone https://github.com/kube-zen/{{ .projectName }}.git
cd {{ .projectName }}
go mod download
```

## Quick Start

```bash
# Run all checks
make check

# Run tests
go test ./...

# Build
go build ./cmd/{{ .projectName }}
```

## Development Workflow

1. Create a feature branch from `main`
2. Make your changes
3. Run `make check` to ensure all checks pass
4. Commit and push your changes
5. Open a pull request

## Testing

```bash
# Run unit tests
go test ./...

# Run tests with race detector
go test -race ./...

# Run specific test
go test -v ./pkg/...
```

## Building

```bash
# Build binary
make build

# Build Docker image
make build-image
```

## Code Standards

- Follow Go best practices
- Run `go fmt` before committing
- Ensure all tests pass
- Add tests for new functionality

## Go Toolchain (S133)

### Version

- **Go 1.24** is the standard across all OSS repos
- Specified in `go.mod`: `go 1.24`
- Toolchain directive: Either use `toolchain go1.24.0` everywhere or nowhere (OSS consistency)

### go.mod Requirements

- Run `go mod tidy` regularly
- No `replace` directives in main branch (unless explicitly required for local dev)
- Pin dependency versions (no pseudo-versions in production)

### Standard Commands

```bash
# Test
go test ./...

# Test with race detector
go test -race ./...

# Build
go build ./...

# Format
gofmt -s -w .
goimports -w .

# Lint
golangci-lint run
```

## Architecture Notes

### Refactoring History

The codebase has been refactored to improve testability and reduce complexity:

1. **PolicyEvaluationService**: Extracted policy evaluation logic into a service that can be easily mocked for testing
2. **Helper Functions**: Extracted complex logic from `Reconcile()` into focused helper functions
3. **GVRResolver**: Created with RESTMapper support for reliable GroupVersionResource resolution
4. **Deprecated GCController**: Removed in favor of `GCPolicyReconciler` with better testability

**Key Learnings**:
- Mock-based tests are preferred over complex fake client setups
- Helper functions improve testability and reduce cyclomatic complexity
- RESTMapper integration improves reliability for irregular CRDs

### Code Organization

- **Controller Logic**: `pkg/controller/reconciler.go` - Main reconciliation logic
- **Helper Functions**: `pkg/controller/reconciler_helpers.go` - Extracted helper functions
- **Evaluation Service**: `pkg/controller/evaluation_service.go` - Policy evaluation service
- **Testing**: `pkg/controller/testing/` - Mock-based tests

## Release Process

See [RELEASE.md](RELEASE.md) for the release process.

