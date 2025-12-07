# Build the controller binary
FROM golang:1.25 AS builder
ARG TARGETOS
ARG TARGETARCH
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# Copy the go source
COPY cmd/ cmd/
COPY api/ api/
COPY internal/ internal/
COPY vendor/ vendor/

# Build using vendor directory (faster, no download needed)
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -mod=vendor -a -o controller cmd/controller/main.go

# Use distroless as minimal base image to package the controller binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot

# Re-declare ARGs for this stage (ARGs don't persist across stages)
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown

# Add labels for traceability
LABEL org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.created="${BUILD_TIME}" \
      org.opencontainers.image.source="https://github.com/stolostron/kubetask" \
      org.opencontainers.image.title="kubetask-controller" \
      org.opencontainers.image.description="KubeTask Controller - Kubernetes-native AI task execution"

WORKDIR /
COPY --from=builder /workspace/controller .
USER 65532:65532

ENTRYPOINT ["/controller"]
