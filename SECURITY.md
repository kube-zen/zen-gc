# Security Policy

## Supported Versions

We release patches to fix security issues. Which versions are eligible for receiving such patches depends on the CVSS v3.0 Rating:

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |
| < 0.1   | :x:                |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them via one of the following methods:

1. **Email**: security@kube-zen.io (preferred)
2. **GitHub Security Advisory**: Use the "Report a vulnerability" button on the repository's Security tab

### What to Include

When reporting a vulnerability, please include:

- Type of vulnerability (e.g., XSS, SQL injection, etc.)
- Full paths of source file(s) related to the vulnerability
- Location of the affected code (tag/branch/commit or direct URL)
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the issue, including how an attacker might exploit it

### Response Timeline

- **Initial Response**: Within 48 hours
- **Status Update**: Within 7 days
- **Fix Timeline**: Depends on severity and complexity
  - **Critical**: As soon as possible (typically < 7 days)
  - **High**: Within 30 days
  - **Medium**: Within 90 days
  - **Low**: Best effort

### Disclosure Policy

- We will acknowledge receipt of your vulnerability report within 48 hours
- We will provide an estimated timeline for a fix
- We will notify you when the vulnerability is fixed
- We will credit you in the security advisory (if desired)

### Security Best Practices

#### For Users

1. **Keep Updated**: Always use the latest stable version
2. **RBAC**: Use minimal RBAC permissions
3. **Network Policies**: Restrict network access where possible
4. **Audit Logs**: Enable Kubernetes audit logging
5. **Secrets Management**: Use Kubernetes secrets or external secret managers

#### For Developers

1. **Dependencies**: Keep dependencies up to date
2. **Security Scanning**: Run `govulncheck` and `gosec` regularly
3. **Input Validation**: Validate all inputs
4. **Error Handling**: Don't expose sensitive information in errors
5. **Least Privilege**: Use minimal RBAC permissions

### Security Checklist

Before deploying:

- [ ] RBAC permissions reviewed and minimized
- [ ] Security context configured (non-root, read-only filesystem)
- [ ] Network policies applied
- [ ] Secrets properly managed
- [ ] Dependencies scanned for vulnerabilities
- [ ] Audit logging enabled

### Known Security Considerations

1. **RBAC**: Controller requires delete permissions on target resources
2. **Admission Webhooks**: Consider validating policies before creation
3. **Rate Limiting**: Prevents API server overload
4. **Dry Run**: Use for testing policies before enabling

### Security Updates

Security updates will be announced via:
- GitHub Security Advisories
- Release notes
- CHANGELOG.md

---

## Security Contact

- **Email**: security@kube-zen.io
- **GitHub**: Use Security tab on repository

Thank you for helping keep zen-gc secure! 🔒




