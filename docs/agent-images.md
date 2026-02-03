# Agent Images

KubeOpenCode provides **template agent images** that serve as starting points for building your own customized agents. These templates demonstrate the agent interface pattern and include common development tools, but are designed to be customized based on your specific requirements.

## Two-Container Pattern

KubeOpenCode uses a **two-container pattern**:

1. **Init Container** (`agentImage`): Contains the OpenCode CLI, copies it to a shared `/tools` volume
2. **Worker Container** (`executorImage`): Your development environment that uses `/tools/opencode`

## Available Images

| Image | Type | Description |
|-------|------|-------------|
| `opencode` | Init Container | OpenCode CLI binary |
| `devbox` | Worker (Executor) | Universal development environment with Go, Node, Python, kubectl, helm |
| `echo` | Testing | Minimal Alpine image for E2E testing |

## Image Resolution

When configuring an Agent, the controller resolves images as follows:

| Configuration | Init Container | Worker Container |
|--------------|----------------|------------------|
| Both `agentImage` and `executorImage` set | `agentImage` | `executorImage` |
| Only `agentImage` set (legacy) | Default OpenCode image | `agentImage` |
| Neither set | Default OpenCode image | Default devbox image |

### Default Images

- OpenCode init: `quay.io/kubeopencode/kubeopencode-agent-opencode:latest`
- Devbox executor: `quay.io/kubeopencode/kubeopencode-agent-devbox:latest`

## Building Agent Images

### Local Development (Kind Clusters)

```bash
# Build OpenCode init container
make agent-build AGENT=opencode

# Build executor containers
make agent-build AGENT=devbox

# Customize registry and version
make agent-build AGENT=devbox IMG_REGISTRY=docker.io IMG_ORG=myorg VERSION=v1.0.0
```

### Remote/Production Clusters

For remote clusters (OpenShift, EKS, GKE, etc.), use multi-arch build and push:

```bash
# Multi-arch build and push
make agent-buildx AGENT=opencode
make agent-buildx AGENT=devbox
```

## Creating Custom Agent Images

For detailed guidance on building custom agent images, see the [Agent Developer Guide](../agents/README.md).

### Example Custom Executor

```dockerfile
FROM quay.io/kubeopencode/kubeopencode-agent-devbox:latest

# Add your custom tools
RUN apt-get update && apt-get install -y \
    your-custom-tool \
    another-tool

# Add custom configuration
COPY .bashrc /home/agent/.bashrc
```

## Next Steps

- [Getting Started](getting-started.md) - Installation and basic usage
- [Features](features.md) - Context system, concurrency, and more
- [Security](security.md) - RBAC, credential management, and best practices
- [Agent Developer Guide](../agents/README.md) - Detailed agent development
