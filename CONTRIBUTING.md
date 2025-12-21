# Contributing to zen-gc

Thank you for your interest in contributing to zen-gc! This document provides guidelines and instructions for contributing.

## Development Setup

### Prerequisites

- Go 1.23 or later
- kubectl configured to access a Kubernetes cluster
- Docker (for building images)
- Make (for running common tasks)

### Getting Started

1. **Fork and clone the repository:**
   ```bash
   git clone https://github.com/kube-zen/zen-gc.git
   cd zen-gc
   ```

2. **Install dependencies:**
   ```bash
   make deps
   ```

3. **Install development tools:**
   ```bash
   make install-tools
   ```

4. **Install pre-commit hooks (recommended):**
   ```bash
   pip install pre-commit
   pre-commit install
   ```

   This will run checks automatically before each commit, catching issues early.

## Pre-commit Hooks

We use pre-commit hooks to ensure code quality. The hooks run automatically on commit and check:

- Code formatting (gofmt)
- Go vet
- YAML/JSON syntax
- License headers
- Large files
- Merge conflicts
- And more...

To run hooks manually:
```bash
pre-commit run --all-files
```

## Development Workflow

### Making Changes

1. **Create a branch:**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes**

3. **Run tests:**
   ```bash
   make test-unit
   ```

4. **Check code quality:**
   ```bash
   make verify
   make lint
   ```

5. **Commit your changes:**
   ```bash
   git add .
   git commit -m "feat: add your feature"
   ```
   
   Pre-commit hooks will run automatically. If they fail, fix the issues and commit again.

### Commit Messages

We follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation changes
- `refactor:` - Code refactoring
- `test:` - Test changes
- `chore:` - Build/tooling changes

Examples:
```
feat: add support for custom TTL calculations
fix: handle nil pointer in policy evaluation
docs: update architecture documentation
```

### Testing

- **Unit tests:**
  ```bash
  make test-unit
  ```

- **Integration tests:**
  ```bash
  make test-integration
  ```

- **E2E tests (requires cluster):**
  ```bash
  make test-e2e
  ```

- **Coverage:**
  ```bash
  make coverage
  ```

### Building

- **Development build:**
  ```bash
  make build
  ```

- **Optimized release build:**
  ```bash
  make build-release
  ```

- **Docker image:**
  ```bash
  make build-image
  ```

## Code Style

- Follow Go standard formatting (`gofmt`)
- Run `make fmt` before committing
- Follow the linter rules in `.golangci.yml`
- Add comments for exported functions/types
- Keep functions focused and small

## Pull Requests

1. **Ensure your branch is up to date:**
   ```bash
   git checkout main
   git pull origin main
   git checkout your-branch
   git rebase main
   ```

2. **Push your branch:**
   ```bash
   git push origin your-branch
   ```

3. **Create a PR on GitHub**

4. **Ensure CI passes** - All checks must pass before merge

### PR Checklist

- [ ] Code follows style guidelines
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] Commit messages follow conventions
- [ ] Pre-commit hooks pass
- [ ] CI checks pass

## Reporting Issues

When reporting bugs or requesting features:

1. Check existing issues first
2. Use the issue templates
3. Provide:
   - Clear description
   - Steps to reproduce (for bugs)
   - Expected vs actual behavior
   - Environment details (Kubernetes version, etc.)

## Code Review

- Be respectful and constructive
- Focus on the code, not the person
- Ask questions if something is unclear
- Suggest improvements, don't just point out problems

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.

## Questions?

- Check the [documentation](docs/)
- Open an issue for questions
- Join our community discussions

Thank you for contributing! 🎉
