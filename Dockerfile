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

# Build stage
# Pin base image with SHA digest for security
# To get SHA: docker pull golang:1.25-alpine && docker inspect golang:1.25-alpine | grep RepoDigests
# Or check: https://hub.docker.com/r/library/golang/tags
FROM golang:1.25-alpine@sha256:ef75fa8822a4c0fb53a390548b3dc1c39639339ec3373c58f5441117e1ff46ae AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git make

# Copy zen-sdk first (needed for tagged version resolution during build)
# Build context should be from parent directory (zen/)
COPY zen-sdk /build/zen-sdk

# Copy go mod files
COPY zen-gc/go.mod zen-gc/go.sum* ./

# Download dependencies first (will fail for zen-sdk but that's ok)
RUN go mod download || true

# Add temporary replace directive for local build (go.mod stays clean with tagged version)
# Path is relative to /build (where go.mod is)
RUN go mod edit -replace github.com/kube-zen/zen-sdk=./zen-sdk

# Copy source code
COPY zen-gc/ .

# Add replace directive to use local zen-sdk during build
RUN go mod edit -replace github.com/kube-zen/zen-sdk=./zen-sdk

# Download dependencies (replace directive handles zen-sdk)
# Use || true to allow partial success
RUN go mod download || true

# Build optimized binary
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE
ARG TARGETOS=linux
ARG TARGETARCH=amd64

# Build for target architecture (defaults to linux/amd64 for single-arch builds)
# Use -mod=mod to allow automatic dependency fetching (replace directive handles zen-sdk)
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -mod=mod -trimpath \
    -ldflags "-s -w \
        -X 'main.version=${VERSION}' \
        -X 'main.commit=${COMMIT}' \
        -X 'main.buildDate=${BUILD_DATE}'" \
    -o gc-controller ./cmd/gc-controller

# Runtime stage - use scratch (empty) base for minimal size
# The binary is statically linked (CGO_ENABLED=0), so no libc needed
FROM scratch

# Copy CA certificates from Alpine for HTTPS/TLS support (needed for Kubernetes API)
# This is much smaller than the full Alpine base (~200KB vs 8MB)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /build/gc-controller /gc-controller

EXPOSE 8080

ENTRYPOINT ["/gc-controller"]

