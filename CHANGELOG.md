# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Go 1.25 Upgrade**: Upgraded to Go 1.25.0 for improved performance and new features

### Added
- Leader election support for HA deployments
- Proper CRD status updates (replaces TODO)
- Kubernetes events for policy lifecycle and resource deletions
- Exponential backoff retry logic for transient errors
- Comprehensive test suite (>75% coverage)
- CI/CD pipeline with GitHub Actions
- golangci-lint configuration
- CONTRIBUTING.md guide
- SECURITY.md policy
- CODE_OF_CONDUCT.md

### Changed
- Updated deployment manifests for HA (2 replicas)
- Enhanced RBAC with lease and event permissions
- Improved error handling with exponential backoff

### Fixed
- Status updates now properly update CRD status subresource
- Error handling for transient API server errors

## [0.0.1-alpha] - 2025-12-24

### Changed
- Updated chart version to 0.0.1-alpha for alpha release
- Default image tag set to `latest` for easier development

## [0.1.0] - 2025-12-21

### Added
- Initial implementation of GC controller
- GarbageCollectionPolicy CRD
- Fixed and dynamic TTL support
- Label and field selector support
- Condition evaluation (phase, labels, annotations, field conditions)
- Rate limiting and batching
- Dry-run mode
- Prometheus metrics
- Basic deployment manifests
- Documentation (KEP, API reference, user guide, operator guide)

---

## Release Process

1. Update version in `go.mod` and `deploy/manifests/deployment.yaml`
2. Update CHANGELOG.md with release notes
3. Create git tag: `git tag -a v0.1.0 -m "Release v0.1.0"`
4. Push tag: `git push origin v0.1.0`
5. Create GitHub release with release notes

---

## Versioning

- **MAJOR** version for incompatible API changes
- **MINOR** version for new functionality in a backward compatible manner
- **PATCH** version for backward compatible bug fixes

---

## Links

- [GitHub Releases](https://github.com/kube-zen/zen-gc/releases)
- [KEP Document](docs/KEP_GENERIC_GARBAGE_COLLECTION.md)

