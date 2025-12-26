.PHONY: build test test-unit test-integration test-e2e fmt vet lint clean deploy coverage verify ci-check security-check

# Build the gc-controller binary (development build with basic optimizations)
build:
	@echo "Building gc-controller..."
	go build -ldflags="-s -w" -trimpath -o bin/gc-controller ./cmd/gc-controller
	@echo "✅ Build complete: bin/gc-controller"
	@ls -lh bin/gc-controller | awk '{print "   Binary size: " $$5}'

# Build optimized binary for production
build-release:
	@echo "Building optimized gc-controller binary..."
	@VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "dev"); \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	BUILD_DATE=$$(date -u +"%Y-%m-%dT%H:%M:%SZ"); \
	go build -trimpath \
		-ldflags "-s -w \
			-X 'main.version=$$VERSION' \
			-X 'main.commit=$$COMMIT' \
			-X 'main.buildDate=$$BUILD_DATE'" \
		-o bin/gc-controller ./cmd/gc-controller
	@echo "✅ Optimized build complete: bin/gc-controller"
	@ls -lh bin/gc-controller

# Build Docker image (requires Docker)
build-image:
	@echo "Building Docker image..."
	@VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "dev"); \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	BUILD_DATE=$$(date -u +"%Y-%m-%dT%H:%M:%SZ"); \
	docker build \
		--build-arg VERSION=$$VERSION \
		--build-arg COMMIT=$$COMMIT \
		--build-arg BUILD_DATE=$$BUILD_DATE \
		-t kube-zen/gc-controller:$$VERSION \
		-t kube-zen/gc-controller:latest .
	@echo "✅ Docker image built: kube-zen/gc-controller:$$VERSION"

# Build multi-arch Docker images (requires Docker Buildx)
build-image-multiarch:
	@echo "Building multi-arch Docker images..."
	@VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "dev"); \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	BUILD_DATE=$$(date -u +"%Y-%m-%dT%H:%M:%SZ"); \
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$$VERSION \
		--build-arg COMMIT=$$COMMIT \
		--build-arg BUILD_DATE=$$BUILD_DATE \
		-t kube-zen/gc-controller:$$VERSION \
		-t kube-zen/gc-controller:latest \
		--push .
	@echo "✅ Multi-arch Docker images built: kube-zen/gc-controller:$$VERSION"

# Run all tests
test: test-unit test-integration

# Run unit tests
test-unit:
	@echo "Running unit tests..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic -timeout=10m ./pkg/...

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	go test -v -timeout=5m ./test/integration/...

# Run E2E tests (requires Kubernetes cluster)
# Usage: make test-e2e CLUSTER_NAME=zen-gc-e2e
test-e2e:
	@echo "Running E2E tests..."
	@if [ -z "$(CLUSTER_NAME)" ]; then \
		echo "⚠️  No cluster specified. Using default or existing cluster."; \
	fi
	go test -v -tags=e2e -timeout=30m ./test/e2e/...

# Setup E2E test cluster with kind
test-e2e-setup:
	@echo "Setting up E2E test cluster..."
	@cd test/e2e && ./setup_kind.sh create

# Cleanup E2E test cluster
test-e2e-cleanup:
	@echo "Cleaning up E2E test cluster..."
	@cd test/e2e && ./setup_kind.sh delete

# Validate example policies
validate-examples:
	@echo "Validating example policies..."
	@go build -o bin/validate-examples ./cmd/validate-examples
	@./bin/validate-examples -dir examples

# Run load tests
test-load:
	@echo "Running load tests..."
	@cd test/load && ./load_test.sh

# Run automated load tests with performance regression detection
test-load-automated:
	@echo "Running automated load tests..."
	@cd test/load && ./load_test_automated.sh --all

# Show test coverage
coverage: test-unit
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report generated: coverage.html"
	@echo "Coverage summary:"
	@go tool cover -func=coverage.out | tail -1
	@echo ""
	@echo "Checking coverage threshold (minimum: 75%)..."
	@COVERAGE=$$(go tool cover -func=coverage.out | tail -1 | awk '{print $$3}' | sed 's/%//'); \
	if [ -z "$$COVERAGE" ]; then \
		echo "⚠️  Could not determine coverage percentage"; \
	elif [ $$(echo "$$COVERAGE < 75" | bc -l) -eq 1 ]; then \
		echo "❌ Coverage $$COVERAGE% is below the 75% threshold"; \
		exit 1; \
	else \
		echo "✅ Coverage $$COVERAGE% meets the 75% threshold"; \
	fi

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "✅ Code formatted"

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...
	@echo "✅ go vet passed"

# Run linter (requires golangci-lint)
lint:
	@echo "Running golangci-lint..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "⚠️  golangci-lint not found. Installing..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin latest; \
	fi
	golangci-lint run
	@echo "✅ Linting passed"

# Security checks
security-check:
	@echo "Running security checks..."
	@if ! command -v govulncheck >/dev/null 2>&1; then \
		echo "Installing govulncheck..."; \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi
	govulncheck ./...
	@echo "✅ Security check passed"

# Check formatting
check-fmt:
	@echo "Checking code formatting..."
	@if [ "$$(gofmt -s -l . | wc -l)" -gt 0 ]; then \
		echo "❌ Code is not formatted. Run 'make fmt'"; \
		gofmt -s -d .; \
		exit 1; \
	fi
	@echo "✅ Code formatting check passed"

# Check go mod tidy
check-mod:
	@echo "Checking go.mod..."
	@go mod tidy
	@if ! git diff --exit-code go.mod go.sum >/dev/null 2>&1; then \
		echo "❌ go.mod or go.sum needs updates. Run 'go mod tidy'"; \
		git diff go.mod go.sum; \
		exit 1; \
	fi
	@echo "✅ go.mod check passed"

# Verify code compiles
verify: check-fmt check-mod vet
	@echo "Verifying code compiles..."
	go build ./...
	@echo "✅ Code compiles successfully"

# CI check (runs all checks)
ci-check: verify lint test-unit security-check
	@echo "✅ All CI checks passed"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/ coverage.out coverage.html
	@echo "✅ Clean complete"

# Deploy CRD
deploy-crd:
	@echo "Deploying CRD..."
	kubectl apply -f deploy/crds/
	@echo "✅ CRD deployed"

# Run controller locally (requires kubeconfig)
run:
	@echo "Running controller locally..."
	go run ./cmd/gc-controller

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy
	@echo "✅ Dependencies installed"

# Install development tools
install-tools:
	@echo "Installing development tools..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin latest; \
	fi
	@if ! command -v govulncheck >/dev/null 2>&1; then \
		echo "Installing govulncheck..."; \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi
	@if ! command -v helm >/dev/null 2>&1; then \
		echo "⚠️  Helm not found. Install from https://helm.sh/docs/intro/install/"; \
	fi
	@echo "✅ Development tools installed"

# Helm chart targets
helm-lint:
	@echo "Linting Helm chart..."
	@if ! command -v helm >/dev/null 2>&1; then \
		echo "❌ Helm not installed. Install from https://helm.sh/docs/intro/install/"; \
		exit 1; \
	fi
	helm lint charts/gc-controller
	@echo "✅ Helm chart lint passed"

helm-package:
	@echo "Packaging Helm chart..."
	@if ! command -v helm >/dev/null 2>&1; then \
		echo "❌ Helm not installed. Install from https://helm.sh/docs/intro/install/"; \
		exit 1; \
	fi
	@mkdir -p .helm-packages
	helm package charts/gc-controller -d .helm-packages/
	@echo "✅ Helm chart packaged"

helm-test:
	@echo "Testing Helm chart rendering..."
	@if ! command -v helm >/dev/null 2>&1; then \
		echo "❌ Helm not installed. Install from https://helm.sh/docs/intro/install/"; \
		exit 1; \
	fi
	helm template test-release charts/gc-controller --debug > /dev/null
	@echo "✅ Helm chart renders successfully"

helm-repo-index:
	@echo "Generating Helm repository index..."
	@if ! command -v helm >/dev/null 2>&1; then \
		echo "❌ Helm not installed. Install from https://helm.sh/docs/intro/install/"; \
		exit 1; \
	fi
	@if [ ! -d ".helm-packages" ] || [ -z "$$(ls -A .helm-packages/*.tgz 2>/dev/null)" ]; then \
		echo "❌ No packaged charts found. Run 'make helm-package' first"; \
		exit 1; \
	fi
	helm repo index .helm-packages --url https://kube-zen.github.io/zen-gc
	@echo "✅ Helm repository index generated"

helm-all: helm-lint helm-test helm-package helm-repo-index
	@echo "✅ All Helm tasks complete"
