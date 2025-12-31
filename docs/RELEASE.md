# Release Process

This document describes the release process for {{ .projectName }}.

## Versioning

We follow [Semantic Versioning](https://semver.org/):
- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes (backward compatible)

## Release Steps (S131)

1. **Update CHANGELOG.md** with release notes
2. **Create a git tag:**
   ```bash
   git tag -a v0.0.1-alpha -m "Release v0.0.1-alpha"
   git push origin v0.0.1-alpha
   ```

3. **GitHub Actions will automatically:**
   - Build and push Docker images (if applicable)
   - Create GitHub release
   - Publish Helm charts (if applicable)

## Tag → Artifact Mapping

### Libraries (zen-sdk)
- Tag: `v0.1.0-alpha`
- Artifact: Go module version
- Changelog: Required entry in CHANGELOG.md

### Controllers (zen-flow, zen-gc, zen-watcher, zen-lock, zen-lead)
- Tag: `v0.0.1-alpha`
- Image: `kubezen/<project-name>:v0.0.1-alpha` (from project.yaml.imageName)
- Changelog: Required entry in CHANGELOG.md

### Charts (helm-charts)
- Tag: Chart version (e.g., `v0.0.2-alpha`)
- Artifact: Helm chart package
- Matrix: See `docs/RELEASE_VERSION_MATRIX.md` for canonical mapping

## Pre-Release Checklist

- [ ] All tests pass
- [ ] CHANGELOG.md updated
- [ ] Version bumped in relevant files
- [ ] Documentation updated
- [ ] Security scan passed

## Post-Release

- [ ] Verify release artifacts
- [ ] Announce release (if applicable)
- [ ] Update downstream dependencies

