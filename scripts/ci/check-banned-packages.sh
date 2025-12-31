#!/bin/bash
# Copyright 2025 Kube-ZEN Contributors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# H116: CI guard to prevent re-duplication of shared capabilities
# This script checks for banned package paths that must live in zen-sdk

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# Call zen-sdk's check-banned-packages.sh if available
if [ -f "${REPO_ROOT}/zen-sdk/scripts/ci/check-banned-packages.sh" ]; then
    exec "${REPO_ROOT}/zen-sdk/scripts/ci/check-banned-packages.sh"
fi

# Fallback: local check
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "H116: Check for Banned Package Paths (Re-Duplication Guard)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

FAILED=0

# Check for banned paths in this repo
BANNED_PATHS=(
    "internal/gc"
    "pkg/gc"
    "pkg/ratelimiter"
    "pkg/backoff"
    "pkg/fieldpath"
    "pkg/ttl"
    "pkg/selector"
)

for banned_path in "${BANNED_PATHS[@]}"; do
    if [ -d "${SCRIPT_DIR}/../../${banned_path}" ]; then
        # Check if it's test code or examples (allowed)
        if find "${SCRIPT_DIR}/../../${banned_path}" -name "*.go" ! -name "*_test.go" ! -name "example*" | grep -q .; then
            echo "❌ Found banned path: ${banned_path}"
            echo "   Must use: zen-sdk/pkg/gc/*"
            FAILED=1
        fi
    fi
done

if [ ${FAILED} -eq 0 ]; then
    echo "✅ No banned package paths found"
    exit 0
else
    echo "❌ Found violations. Shared capabilities must be in zen-sdk."
    exit 1
fi

