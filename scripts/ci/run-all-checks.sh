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

# H120: Run all CI checks (functional tests, race detection, banned packages)
# This script should be called in CI pipelines

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Running All CI Checks (H120)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

cd "${REPO_ROOT}/zen-gc"

FAILED=0

# 1. Functional tests
echo "1. Running functional tests..."
if ! go test ./... -timeout 5m; then
    echo "❌ Functional tests failed"
    FAILED=1
else
    echo "✅ Functional tests passed"
fi

echo ""

# 2. Race detection (targeted to GC primitives)
echo "2. Running race detection tests..."
if ! bash "${SCRIPT_DIR}/test-with-race.sh"; then
    echo "❌ Race detection tests failed"
    FAILED=1
else
    echo "✅ Race detection tests passed"
fi

echo ""

# 3. Banned packages check
echo "3. Checking for banned package paths..."
if ! bash "${SCRIPT_DIR}/check-banned-packages.sh"; then
    echo "❌ Banned packages check failed"
    FAILED=1
else
    echo "✅ Banned packages check passed"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ ${FAILED} -eq 0 ]; then
    echo "✅ All CI checks passed"
    exit 0
else
    echo "❌ Some CI checks failed"
    exit 1
fi

