.PHONY: build test test-unit test-integration test-e2e fmt vet lint clean deploy coverage

# Build the gc-controller binary
build:
	go build -o bin/gc-controller ./cmd/gc-controller

# Run all tests
test: test-unit test-integration

# Run unit tests
test-unit:
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./pkg/...

# Run integration tests
test-integration:
	go test -v ./test/integration/...

# Run E2E tests (requires Kubernetes cluster)
test-e2e:
	go test -v -tags=e2e ./test/e2e/...

# Show test coverage
coverage: test-unit
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run

# Clean build artifacts
clean:
	rm -rf bin/ coverage.out coverage.html

# Deploy CRD
deploy-crd:
	kubectl apply -f deploy/crds/

# Run controller locally (requires kubeconfig)
run:
	go run ./cmd/gc-controller

# Install dependencies
deps:
	go mod download
	go mod tidy

# Verify code compiles
verify:
	go build ./...
