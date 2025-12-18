# Copyright Contributors to the KubeTask project
# git-init - Lightweight Git clone init container for KubeTask
#
# This image provides a simple git clone capability for Git Context.
# It replaces the external git-sync dependency with a self-contained solution.
#
# Size: ~15-20MB (alpine + git + openssh + go binary)

# Build stage
FROM golang:1.25-alpine AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.mod
COPY go.sum go.sum

# Copy the git-init source
COPY cmd/git-init/main.go cmd/git-init/main.go

# Build the binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -ldflags="-s -w" -o git-init cmd/git-init/main.go

# Runtime stage - use alpine for git and ssh
FROM alpine:3.21

# Install git and ssh client for repository cloning
RUN apk add --no-cache \
    git \
    openssh-client \
    && rm -rf /var/cache/apk/*

# Add labels for traceability
LABEL org.opencontainers.image.source="https://github.com/kubetask-io/kubetask" \
      org.opencontainers.image.title="kubetask-git-init" \
      org.opencontainers.image.description="KubeTask Git Init - Git clone init container for Git Context"

# Copy the binary from builder
COPY --from=builder /workspace/git-init /git-init

# Create the default git root directory
RUN mkdir -p /git && chmod 777 /git

# Run as non-root user for security
RUN adduser -D -u 65532 git-init
USER 65532

ENTRYPOINT ["/git-init"]
