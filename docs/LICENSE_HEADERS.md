# License Headers

All source files should include Apache 2.0 license headers.

## Header Format

### Go Files

```go
/*
Copyright 2025 Kube-ZEN Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller
```

### YAML Files

```yaml
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
```

## Adding Headers

You can use a tool like `addlicense`:

```bash
go install github.com/google/addlicense@latest
addlicense -c "Kube-ZEN Contributors" -l apache .
```

## Exceptions

The following files don't need headers:
- Generated files (`zz_generated.deepcopy.go`)
- Configuration files (`.golangci.yml`, `.gitignore`)
- Documentation files (`.md` files)
- Test fixtures

---

**Note**: License headers are recommended but not strictly required for Apache 2.0. The LICENSE file in the root is sufficient for compliance.




