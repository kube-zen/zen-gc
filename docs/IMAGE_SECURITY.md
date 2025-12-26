# Image Security Documentation

This document describes image security practices for zen-gc, including base image pinning, security scanning, and update procedures.

## Base Image Pinning

All base images in the Dockerfile are pinned using SHA digests for security and reproducibility.

### Current Base Images

**Build Stage:**
```dockerfile
FROM golang:1.24-alpine@sha256:<SHA>
```

**Runtime Stage:**
```dockerfile
FROM alpine:3.19@sha256:<SHA>
```

### Why SHA Pinning?

1. **Security**: Prevents supply chain attacks by ensuring exact image version
2. **Reproducibility**: Same SHA = same image, regardless of when it's pulled
3. **Auditability**: Easy to verify which exact image is being used
4. **Stability**: Prevents unexpected changes from image updates

### Finding Image SHAs

To find the SHA digest for an image:

```bash
# Pull the image
docker pull alpine:3.19

# Inspect to get SHA
docker inspect alpine:3.19 | grep -A 5 RepoDigests

# Or use docker manifest
docker manifest inspect alpine:3.19 | grep -A 10 digest
```

**Online:**
- Visit https://hub.docker.com/r/library/alpine/tags
- Click on the specific tag (e.g., `3.19`)
- Find the SHA256 digest in the image details

## Security Scanning

### CI/CD Scanning

The CI/CD pipeline includes multiple security scanning tools:

#### 1. Trivy (Docker Image Scanning)

**When**: Runs on every PR and push to main

**What it scans**:
- Base image vulnerabilities
- Installed packages (apk packages in Alpine)
- Go binary dependencies (if applicable)

**Configuration**:
- Severity threshold: `CRITICAL,HIGH`
- Format: SARIF (uploaded to GitHub Security tab)
- Exit code: `0` (reports but doesn't fail CI)

**View Results**:
- GitHub Security tab → Code scanning alerts
- Trivy output in CI logs

#### 2. govulncheck (Go Vulnerabilities)

**When**: Runs on every CI run

**What it scans**:
- Go module dependencies
- Known vulnerabilities in Go standard library

**Configuration**:
- Scans all Go packages (`./...`)
- Uses Go vulnerability database

#### 3. gosec (Security Linter)

**When**: Runs on every CI run

**What it scans**:
- Hardcoded secrets
- Weak cryptographic functions
- SQL injection risks
- Insecure random number generation
- And more...

**Configuration**:
- JSON output format
- Results uploaded as artifact

### Local Security Scanning

#### Scan Docker Image Locally

```bash
# Install Trivy
brew install trivy  # macOS
# or
sudo apt-get install trivy  # Ubuntu/Debian

# Build image
make build-image

# Scan image
trivy image kube-zen/gc-controller:latest

# Scan with specific severity
trivy image --severity CRITICAL,HIGH kube-zen/gc-controller:latest

# Scan and save report
trivy image --format json --output trivy-report.json kube-zen/gc-controller:latest
```

#### Scan Go Dependencies

```bash
# Install govulncheck
go install golang.org/x/vuln/cmd/govulncheck@latest

# Scan all packages
govulncheck ./...

# Scan specific package
govulncheck ./pkg/controller
```

#### Run gosec

```bash
# Install gosec
go install github.com/securego/gosec/v2/cmd/gosec@latest

# Scan all packages
gosec ./...

# Scan with JSON output
gosec -fmt json -out gosec-report.json ./...
```

## Image Update Process

### When to Update

1. **Security Vulnerabilities**: When base images have critical/high vulnerabilities
2. **Regular Maintenance**: Monthly or quarterly updates
3. **Feature Requirements**: When new features require newer base images

### Update Procedure

#### Step 1: Check Current Versions

```bash
# Check current SHAs in Dockerfile
grep "^FROM" Dockerfile

# Check for vulnerabilities
trivy image alpine:3.19
trivy image golang:1.24-alpine
```

#### Step 2: Find Latest Secure Versions

```bash
# Check latest Alpine version
docker pull alpine:latest
docker inspect alpine:latest | grep RepoDigests

# Check latest Go Alpine version
docker pull golang:1.24-alpine
docker inspect golang:1.24-alpine | grep RepoDigests

# Or check specific versions
docker pull alpine:3.20
docker inspect alpine:3.20 | grep RepoDigests
```

#### Step 3: Scan New Images

```bash
# Scan new base image before updating
trivy image alpine:3.20
trivy image golang:1.24-alpine@sha256:<NEW_SHA>
```

#### Step 4: Update Dockerfile

```dockerfile
# Update both build and runtime stages
FROM golang:1.24-alpine@sha256:<NEW_SHA> AS builder
FROM alpine:3.20@sha256:<NEW_SHA>
```

#### Step 5: Test Build

```bash
# Build and test
make build-image

# Run security scan
trivy image kube-zen/gc-controller:test

# Test the image
docker run --rm kube-zen/gc-controller:test --help
```

#### Step 6: Update Documentation

- Update this document with new SHAs
- Update CHANGELOG.md with image updates
- Note any breaking changes or compatibility issues

### Automated Update Checks

The CI pipeline includes a step to check for outdated base images:

```yaml
- name: Check for outdated base images
  run: |
    echo "Checking for outdated base images..."
    grep "^FROM" Dockerfile
```

**Note**: This is informational only. Manual review and update is still required.

## Security Best Practices

### 1. Regular Updates

- **Base Images**: Update monthly or when vulnerabilities are found
- **Go Version**: Update when new Go versions are released (check compatibility)
- **Alpine Packages**: Update `ca-certificates` and `tzdata` regularly

### 2. Minimal Base Images

- ✅ Use Alpine Linux (minimal attack surface)
- ✅ Only install necessary packages
- ✅ Remove build dependencies in final image

### 3. Multi-Stage Builds

- ✅ Use separate build and runtime stages
- ✅ Copy only necessary files to final image
- ✅ Don't include source code or build tools in runtime image

### 4. Non-Root User

- ✅ Run as non-root user (`nobody`, UID 65534)
- ✅ Set proper file permissions
- ✅ Use read-only filesystem where possible

### 5. Image Signing (Future)

Consider implementing image signing with:
- **Cosign**: Sign images with cryptographic signatures
- **Notary**: Image signing and verification
- **Docker Content Trust**: Enable content trust for image pulls

## Vulnerability Response

### Critical/High Vulnerabilities

1. **Immediate Action**: Update base image or apply patches
2. **Notify**: Update security advisory if needed
3. **Test**: Thoroughly test updated image
4. **Release**: Create patch release with updated image

### Medium/Low Vulnerabilities

1. **Plan**: Schedule update in next maintenance window
2. **Monitor**: Track vulnerability status
3. **Document**: Note in security documentation

### Reporting Vulnerabilities

If you discover a vulnerability:

1. **Do NOT** create a public issue
2. Email: security@kube-zen.io
3. Or use GitHub Security tab → Report a vulnerability

## Monitoring

### GitHub Security Tab

- View all security alerts
- Track vulnerability status
- Manage security advisories

### Dependabot

GitHub Dependabot can be configured to:
- Monitor Go dependencies
- Create PRs for security updates
- Alert on vulnerabilities

**Example Configuration** (`.github/dependabot.yml`):

```yaml
version: 2
updates:
  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"
    open-pull-requests-limit: 10
```

## References

- [Docker Security Best Practices](https://docs.docker.com/engine/security/)
- [Alpine Linux Security](https://alpinelinux.org/security/)
- [Trivy Documentation](https://aquasecurity.github.io/trivy/)
- [Go Vulnerability Database](https://pkg.go.dev/vuln)
- [gosec Documentation](https://github.com/securego/gosec)

