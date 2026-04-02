# Copyright Contributors to the KubeOpenCode project
#
# Simple echo agent for E2E testing
# This agent simply executes shell commands and echoes task content
#
# Uses Debian (not Alpine) because the OpenCode init container copies a glibc-linked
# binary to /tools/opencode. Alpine uses musl libc, which causes "not found" errors.

FROM debian:bookworm-20260316-slim

# Install minimal tools needed for testing
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    coreutils \
    findutils \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN useradd -u 65532 -m agent

# Create workspace directory
RUN mkdir -p /workspace && chown agent:agent /workspace

USER agent
WORKDIR /workspace

# Default entrypoint - can be overridden by Agent.spec.command
ENTRYPOINT ["/bin/sh", "-c"]
CMD ["echo 'Echo agent ready'"]
