# Goose Agent Image
#
# Goose is an open-source AI agent by Block that runs locally.
# This image runs Goose in headless mode for automated task execution.
#
# Documentation:
# - Goose Homepage: https://block.github.io/goose/
# - Running Tasks: https://block.github.io/goose/docs/guides/running-tasks
# - Headless Mode: https://block.github.io/goose/docs/tutorials/headless-goose/
# - Providers: https://block.github.io/goose/docs/getting-started/providers
# - GitHub: https://github.com/block/goose
#
# Environment Variables (set at runtime via KubeTask Agent secrets):
# - GOOSE_PROVIDER: AI provider (anthropic, openai, google, etc.)
# - GOOSE_MODEL: Model name (e.g., claude-sonnet-4-20250514)
# - API keys: ANTHROPIC_API_KEY, OPENAI_API_KEY, etc.
#
FROM debian:bookworm-slim

# Install git, jq, gh, yq, kubectl, make, procps, gcc and other common tools needed for repository operations
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    jq \
    ca-certificates \
    curl \
    gpg \
    procps \
    make \
    gcc \
    libc6-dev \
    && mkdir -p /etc/apt/keyrings \
    && curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | gpg --dearmor -o /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" > /etc/apt/sources.list.d/github-cli.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends gh \
    && curl -fsSL https://github.com/mikefarah/yq/releases/latest/download/yq_linux_$(dpkg --print-architecture) -o /usr/local/bin/yq \
    && chmod +x /usr/local/bin/yq \
    && curl -fsSL "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/$(dpkg --print-architecture | sed 's/aarch64/arm64/' | sed 's/x86_64/amd64/')/kubectl" -o /usr/local/bin/kubectl \
    && chmod +x /usr/local/bin/kubectl \
    && apt-get purge -y gpg \
    && apt-get autoremove -y \
    && rm -rf /var/lib/apt/lists/*

# Install Go
ARG GO_VERSION=1.25.5
RUN ARCH=$(dpkg --print-architecture | sed 's/amd64/amd64/' | sed 's/arm64/arm64/') \
    && curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" -o /tmp/go.tar.gz \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm /tmp/go.tar.gz

# Install Goose CLI
RUN curl -fsSL https://github.com/block/goose/releases/download/stable/download_cli.sh | bash \
    && mv ~/.local/bin/goose /usr/local/bin/goose

# Install golangci-lint
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /usr/local/bin

# Build argument for workspace directory
ARG WORKSPACE_DIR=/workspace

# Set workspace directory as environment variable for runtime use
ENV WORKSPACE_DIR=${WORKSPACE_DIR}

# Set Go environment - use workspace-relative paths to avoid permission issues
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOPATH="${WORKSPACE_DIR}/.go"
ENV GOMODCACHE="${WORKSPACE_DIR}/.gomodcache"
ENV PATH="${GOPATH}/bin:${PATH}"

# Goose environment defaults
# GOOSE_MODE=auto: auto-approve operations (safe in container environment)
# GOOSE_CONTEXT_STRATEGY=summarize: manage context efficiently
# Provider and model are set at runtime via secrets (GOOSE_PROVIDER, GOOSE_MODEL, API keys)
ENV GOOSE_MODE=auto
ENV GOOSE_CONTEXT_STRATEGY=summarize

# Create non-root user for security
RUN useradd -m -s /bin/bash goose
USER goose
WORKDIR ${WORKSPACE_DIR}

# Default entrypoint - run Goose in headless mode
# The agent reads the task from ${WORKSPACE_DIR}/task.md and executes it
# Using --no-session for one-off tasks (no session history clutter)
# Using -t flag to pass the task content as text instruction
ENTRYPOINT ["sh", "-c", "goose run --no-session -t \"$(cat ${WORKSPACE_DIR}/task.md)\""]
