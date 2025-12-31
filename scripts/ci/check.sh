#!/usr/bin/env bash
# OSS Profile: CI hygiene checks
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FAILED=0

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "OSS Profile: CI Checks"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# S134: Check for private references (early gate, hard fail)
echo ""
echo "Checking for private references..."
if [ -f "$SCRIPT_DIR/scripts/ci/check-no-private-refs.sh" ]; then
    if ! "$SCRIPT_DIR/scripts/ci/check-no-private-refs.sh"; then
        echo "  ❌ Private reference check failed"
        FAILED=1
    else
        echo "  ✅ No private references detected"
    fi
else
    echo "  ⚠ check-no-private-refs.sh not found (skipping)"
fi

# Check for required files
echo ""
echo "Checking required files..."
REQUIRED_FILES=(
    "README.md"
    "SECURITY.md"
    "CONTRIBUTING.md"
    "CODE_OF_CONDUCT.md"
    "CHANGELOG.md"
    "LICENSE"
    "project.yaml"
)

for file in "${REQUIRED_FILES[@]}"; do
    if [ -f "$SCRIPT_DIR/$file" ]; then
        echo "  ✅ $file"
    else
        echo "  ❌ Missing: $file"
        FAILED=1
    fi
done

# Check for .github/workflows
echo ""
echo "Checking GitHub workflows..."
if [ -d "$SCRIPT_DIR/.github/workflows" ]; then
    WORKFLOW_FILES=$(find "$SCRIPT_DIR/.github/workflows" -type f \( -name "*.yml" -o -name "*.yaml" \) 2>/dev/null | wc -l)
    if [ "$WORKFLOW_FILES" -eq 0 ]; then
        echo "  ❌ No workflow files found"
        FAILED=1
    else
        echo "  ✅ Found $WORKFLOW_FILES workflow file(s)"
    fi
else
    echo "  ❌ .github/workflows directory not found"
    FAILED=1
fi

# Check for Makefile check target
echo ""
echo "Checking Makefile..."
if [ -f "$SCRIPT_DIR/Makefile" ]; then
    if grep -q "^check:" "$SCRIPT_DIR/Makefile"; then
        echo "  ✅ Makefile has 'check' target"
    else
        echo "  ❌ Makefile missing 'check' target"
        FAILED=1
    fi
else
    echo "  ⚠ Makefile not found (optional for non-Go projects)"
fi

# Go-specific checks
if [ -f "$SCRIPT_DIR/go.mod" ]; then
    echo ""
    echo "Checking Go configuration..."
    
    # Check go version
    GO_VERSION=$(grep "^go " "$SCRIPT_DIR/go.mod" | awk '{print $2}' || echo "")
    if [ -n "$GO_VERSION" ]; then
        echo "  ✅ Go version: $GO_VERSION"
    else
        echo "  ❌ go.mod missing 'go' directive"
        FAILED=1
    fi
    
    # Check .golangci.yml
    if [ -f "$SCRIPT_DIR/.golangci.yml" ]; then
        echo "  ✅ .golangci.yml found"
    else
        echo "  ⚠ .golangci.yml not found (recommended)"
    fi
    
    # S137: Check for replace directives (OSS repos should not have them)
    if grep -q "^replace " "$SCRIPT_DIR/go.mod"; then
        echo "  ❌ go.mod contains 'replace' directive (not allowed in OSS repos)"
        echo "     Use go.work for local development overrides instead"
        FAILED=1
    else
        echo "  ✅ No 'replace' directives in go.mod"
    fi
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ $FAILED -eq 0 ]; then
    echo "✅ All checks passed"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    exit 0
else
    echo "❌ Some checks failed"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    exit 1
fi

