# Release Process

This document describes the release process for zen-gc.

## Release Types

### Pre-release (v0.x.x)

Pre-releases are used during the development phase:
- `v0.1.0`, `v0.2.0`, etc. for minor releases
- `v0.1.1`, `v0.1.2`, etc. for patch releases
- Pre-releases may have breaking changes

### Stable Release (v1.0.0+)

Stable releases follow semantic versioning:
- **Major** (v1.0.0, v2.0.0): Breaking changes
- **Minor** (v1.1.0, v1.2.0): New features, backward compatible
- **Patch** (v1.0.1, v1.0.2): Bug fixes, backward compatible

## Release Checklist

### Before Release

- [ ] All tests pass (`make test`)
- [ ] Code is linted (`make lint`)
- [ ] Security checks pass (`make security-check`)
- [ ] Documentation is up-to-date
- [ ] CHANGELOG.md is updated with release notes
- [ ] Version numbers are updated in:
  - [ ] `go.mod` (if needed)
  - [ ] `charts/gc-controller/Chart.yaml`
  - [ ] `charts/gc-controller/values.yaml` (if needed)
  - [ ] `docs/KEP_GENERIC_GARBAGE_COLLECTION.md` (if needed)

### Creating a Release

1. **Create Release Branch** (for major/minor releases):
   ```bash
   git checkout -b release/vX.Y.0
   ```

2. **Update Version**:
   - Update version in relevant files
   - Commit changes: `git commit -m "chore: bump version to vX.Y.Z"`

3. **Create Git Tag**:
   ```bash
   git tag -a vX.Y.Z -m "Release vX.Y.Z"
   git push origin vX.Y.Z
   ```

4. **Create GitHub Release**:
   - Go to GitHub Releases page
   - Click "Draft a new release"
   - Select the tag
   - Copy release notes from CHANGELOG.md
   - Publish release

5. **Build and Publish** (if applicable):
   ```bash
   make build-release
   make build-image
   # Push Docker image to registry
   ```

### After Release

- [ ] Verify release artifacts are available
- [ ] Update documentation if needed
- [ ] Announce release (if applicable)

## Release Notes

Release notes should include:

- **Added**: New features
- **Changed**: Changes in existing functionality
- **Deprecated**: Soon-to-be removed features
- **Removed**: Removed features
- **Fixed**: Bug fixes
- **Security**: Security fixes

See [CHANGELOG.md](CHANGELOG.md) for the format.

## Hotfixes

For critical bug fixes:

1. Create hotfix branch from the release tag
2. Apply fix
3. Create patch release (e.g., v1.0.1)
4. Merge hotfix back to main

## Semantic Versioning

This project follows [Semantic Versioning](https://semver.org/):

- **MAJOR** version for incompatible API changes
- **MINOR** version for backward-compatible functionality additions
- **PATCH** version for backward-compatible bug fixes

