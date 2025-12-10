# KubeTask Agent Developer Guide

This guide explains how to build custom agent images for KubeTask.

## Overview

KubeTask agent images are container images that execute AI-powered tasks. The provided templates (gemini, goose, echo) serve as **starting points** that you should customize based on your specific requirements.

**Design Philosophy**: Rather than providing a one-size-fits-all agent image, KubeTask encourages you to build purpose-specific images that include exactly the tools and AI CLIs your tasks need. This approach:

- Reduces image size and startup time
- Improves security by minimizing attack surface
- Allows flexibility in AI provider choice
- Enables custom tool configurations

## Available Templates

| Template | Base Image | AI CLI | Included Tools |
|----------|------------|--------|----------------|
| `gemini` | `node:24-slim` | Gemini CLI | git, jq, yq, gh, kubectl, Go, golangci-lint |
| `goose` | `debian:bookworm-slim` | Goose CLI | git, jq, yq, gh, kubectl, Go, golangci-lint |
| `echo` | `alpine:3.21` | None | bash (for testing) |

## Agent Image Requirements

Every agent image must follow these conventions:

1. **Read task from `/workspace/task.md`**: The controller mounts the task description at this path
2. **Work in `/workspace` directory**: All context files are mounted here
3. **Output to stdout/stderr**: Results are captured as Job logs
4. **Exit with appropriate code**: 0 for success, non-zero for failure

## Building Your Own Agent Image

### Step 1: Choose a Base Image

Select a base image appropriate for your AI CLI:

```dockerfile
# For Node.js-based CLIs (Gemini, Claude Code)
FROM node:24-slim

# For Python-based CLIs
FROM python:3.12-slim

# For Go-based CLIs or minimal images
FROM debian:bookworm-slim
```

### Step 2: Install Development Tools

Add the tools your tasks will need:

```dockerfile
# Common development tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    jq \
    ca-certificates \
    curl \
    make \
    && rm -rf /var/lib/apt/lists/*

# GitHub CLI (optional)
RUN curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | gpg --dearmor -o /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" > /etc/apt/sources.list.d/github-cli.list \
    && apt-get update && apt-get install -y gh

# kubectl (optional)
RUN curl -fsSL "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/$(dpkg --print-architecture)/kubectl" -o /usr/local/bin/kubectl \
    && chmod +x /usr/local/bin/kubectl

# yq (optional)
RUN curl -fsSL https://github.com/mikefarah/yq/releases/latest/download/yq_linux_$(dpkg --print-architecture) -o /usr/local/bin/yq \
    && chmod +x /usr/local/bin/yq
```

### Step 3: Install Your AI CLI

Choose and install your preferred AI CLI:

```dockerfile
# Option A: Gemini CLI
RUN npm install -g @google/gemini-cli

# Option B: Claude Code
RUN npm install -g @anthropic-ai/claude-code

# Option C: Goose
RUN curl -fsSL https://github.com/block/goose/releases/download/stable/download_cli.sh | bash \
    && mv ~/.local/bin/goose /usr/local/bin/goose
```

### Step 4: Configure the Workspace

Set up the workspace directory and environment:

```dockerfile
# Build argument for workspace directory
ARG WORKSPACE_DIR=/workspace

# Set workspace directory as environment variable
ENV WORKSPACE_DIR=${WORKSPACE_DIR}

# Create non-root user for security
RUN useradd -m -s /bin/bash agent
USER agent
WORKDIR ${WORKSPACE_DIR}
```

### Step 5: Define the Entrypoint

Create an entrypoint that reads the task and executes it:

```dockerfile
# Gemini example
ENTRYPOINT ["sh", "-c", "gemini --output-format stream-json --yolo -p \"$(cat ${WORKSPACE_DIR}/task.md)\""]

# Claude Code example
ENTRYPOINT ["sh", "-c", "claude -p \"$(cat ${WORKSPACE_DIR}/task.md)\" --dangerously-skip-permissions"]

# Goose example
ENTRYPOINT ["sh", "-c", "goose run --no-session -t \"$(cat ${WORKSPACE_DIR}/task.md)\""]
```

### Complete Example: Custom Claude Agent

```dockerfile
# Custom Claude Code Agent
FROM node:24-slim

# Install development tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    jq \
    ca-certificates \
    curl \
    make \
    && rm -rf /var/lib/apt/lists/*

# Install Claude Code CLI
RUN npm install -g @anthropic-ai/claude-code

# Workspace configuration
ARG WORKSPACE_DIR=/workspace
ENV WORKSPACE_DIR=${WORKSPACE_DIR}

# Security: run as non-root user
RUN useradd -m -s /bin/bash agent
USER agent
WORKDIR ${WORKSPACE_DIR}

# Execute task with Claude Code
ENTRYPOINT ["sh", "-c", "claude -p \"$(cat ${WORKSPACE_DIR}/task.md)\" --dangerously-skip-permissions"]
```

## Environment Variables

### Required by KubeTask

| Variable | Description | Set By |
|----------|-------------|--------|
| `WORKSPACE_DIR` | Workspace directory path | Dockerfile |
| `TASK_NAME` | Name of the Task CR | Controller |
| `TASK_NAMESPACE` | Namespace of the Task CR | Controller |

### AI Provider Credentials

Configure these via the Agent `credentials` field:

| Provider | Environment Variable |
|----------|---------------------|
| Anthropic (Claude) | `ANTHROPIC_API_KEY` |
| Google (Gemini) | `GOOGLE_API_KEY` or Vertex AI credentials |
| OpenAI | `OPENAI_API_KEY` |
| GitHub | `GITHUB_TOKEN` |

Example Agent configuration:

```yaml
apiVersion: kubetask.io/v1alpha1
kind: Agent
metadata:
  name: claude-agent
spec:
  agentImage: myregistry/my-claude-agent:v1.0
  credentials:
    - name: anthropic-key
      secretRef:
        name: ai-credentials
        key: anthropic-api-key
      env: ANTHROPIC_API_KEY
    - name: github-token
      secretRef:
        name: github-credentials
        key: token
      env: GITHUB_TOKEN
```

## Building Agent Images

### Using Make (Recommended)

From the project root directory:

```bash
# Build specific agent (default: gemini)
make agent-build                    # Build gemini agent
make agent-build AGENT=gemini       # Build gemini agent (explicit)
make agent-build AGENT=goose        # Build goose agent
make agent-build AGENT=echo         # Build echo agent

# Push agent image to registry
make agent-push                     # Push gemini agent
make agent-push AGENT=goose         # Push goose agent

# Multi-arch build and push (linux/amd64 and linux/arm64)
make agent-buildx                   # Multi-arch build gemini
make agent-buildx AGENT=goose       # Multi-arch build goose

# List available agents
make -C agents list
```

### From This Directory

```bash
cd agents

# Build gemini agent
make build

# Build specific agent
make AGENT=goose build

# Push to registry
make push

# Multi-arch build and push
make buildx

# List available agents
make list
```

### Image Naming

Default naming: `quay.io/zhaoxue/kubetask-agent-<AGENT>:latest`

Customize with variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `AGENT` | `gemini` | Agent to build |
| `IMG_REGISTRY` | `quay.io` | Container registry |
| `IMG_ORG` | `zhaoxue` | Registry organization |
| `VERSION` | `latest` | Image tag version |

Example:

```bash
make agent-build AGENT=gemini IMG_REGISTRY=docker.io IMG_ORG=myorg VERSION=v1.0.0
# Builds: docker.io/myorg/kubetask-agent-gemini:v1.0.0
```

## Testing Your Agent Image

### Local Testing with Docker

```bash
# Create a test task file
echo "List the files in the current directory" > /tmp/task.md

# Run the agent locally
docker run --rm \
  -v /tmp/task.md:/workspace/task.md:ro \
  -e GOOGLE_API_KEY=$GOOGLE_API_KEY \
  quay.io/zhaoxue/kubetask-agent-gemini:latest
```

### Testing with Kind Cluster

```bash
# Setup e2e environment
make e2e-setup

# Build and load your agent image
make agent-build AGENT=echo
make e2e-agent-load

# Run e2e tests
make e2e-test
```

## Adding a New Agent Template

To contribute a new agent template:

1. Create a new directory: `mkdir agents/<agent-name>`
2. Add a `Dockerfile` following the patterns above
3. Ensure the entrypoint reads from `/workspace/task.md`
4. Update this README with the new agent details
5. Test locally: `make AGENT=<agent-name> agent-build`

## Security Best Practices

1. **Run as non-root**: Always create and use a non-root user
2. **Minimize packages**: Only install what you need
3. **Use specific versions**: Pin base image and tool versions
4. **Clean up**: Remove package caches and temporary files
5. **Credential handling**: Never bake credentials into images; use Kubernetes secrets

## Troubleshooting

### Agent fails to start

Check that:
- The task file is mounted at `/workspace/task.md`
- Required environment variables (API keys) are set
- The user has permission to access the workspace directory

### AI CLI errors

- Verify API credentials are valid
- Check rate limits and quotas
- Review CLI-specific documentation for error codes

### Image build failures

- Ensure base image supports your architecture (amd64/arm64)
- Check network access for package downloads
- Verify Docker/Podman is running
