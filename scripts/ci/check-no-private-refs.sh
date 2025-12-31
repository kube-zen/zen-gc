#!/usr/bin/env bash
# Check for private references in OSS repos
# This script fails if it finds references to private repos or domains

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FAILED=0

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Checking for Private References"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Patterns to search for (private references)
# CUSTOMIZE THESE PATTERNS FOR YOUR ORGANIZATION
# Replace with your private repo/infrastructure names
PRIVATE_PATTERNS=(
    "private-repo-name-1"  # Example: replace with your private repo
    "private-repo-name-2"  # Example: replace with your private repo
    "internal-tooling"     # Example: replace with your internal tooling
)

# Directories to exclude from scanning
EXCLUDE_DIRS=(
    ".git"
    "vendor"
    "third_party"
    "node_modules"
    ".github/workflows"  # Workflows may reference reusable workflows
)

# Files to exclude from scanning (the check script itself)
EXCLUDE_FILES=(
    "scripts/ci/check-no-private-refs.sh"  # Exclude self
)

# Build find exclude arguments
FIND_EXCLUDE=()
for dir in "${EXCLUDE_DIRS[@]}"; do
    FIND_EXCLUDE+=("-not" "-path" "*/${dir}/*")
done

# Search for private references
for pattern in "${PRIVATE_PATTERNS[@]}"; do
    if [ -z "$pattern" ]; then
        continue  # Skip empty patterns
    fi
    
    echo "Checking for: $pattern"
    
    # Search in tracked files (git ls-files)
    if command -v git >/dev/null 2>&1 && [ -d "$SCRIPT_DIR/.git" ]; then
        MATCHES=$(git ls-files | grep -v -f <(printf '%s\n' "${EXCLUDE_FILES[@]}") | xargs grep -l "$pattern" 2>/dev/null || true)
    else
        # Fallback to find if not in git repo
        MATCHES=$(find "$SCRIPT_DIR" -type f "${FIND_EXCLUDE[@]}" -not -path "*/${EXCLUDE_FILES[0]}" -exec grep -l "$pattern" {} \; 2>/dev/null || true)
    fi
    
    if [ -n "$MATCHES" ]; then
        echo "  ❌ Found references to '$pattern' in:"
        echo "$MATCHES" | while read -r file; do
            echo "    - $file"
            # Show context
            grep -n "$pattern" "$file" 2>/dev/null | head -3 | sed 's/^/      /' || true
        done
        FAILED=1
    else
        echo "  ✅ No references found"
    fi
    echo ""
done

# Check for private domains/URLs (customize per organization)
echo "Checking for private domains..."
# Example: Check for private GitHub orgs/repos
# PRIVATE_DOMAINS=("private-org" "internal-repo")
# Uncomment and customize as needed

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ $FAILED -eq 0 ]; then
    echo "✅ No private references found"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    exit 0
else
    echo "❌ Private references detected - PR blocked"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "Please remove all private references detected above"
    exit 1
fi

