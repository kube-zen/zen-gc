#!/bin/bash
set -e

export PATH=$PATH:/root/go/bin

# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run --timeout=5m

