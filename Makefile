.PHONY: build test fmt vet lint clean deploy

# Build the gc-controller binary
build:
	go build -o bin/gc-controller ./cmd/gc-controller

# Run tests
test:
	go test ./...

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
	rm -rf bin/

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

